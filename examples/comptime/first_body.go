package comptime

import (
	"fmt"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/ssa"
)

// body checks every instruction of a function against the allow-list.
func (j *judge) body(f *Function) {
	for _, block := range f.Body.Blocks {
		for _, instr := range block.Instrs {
			if j.reason != nil {
				return
			}
			if pos := instr.Pos(); pos.IsValid() {
				j.at = pos
			}
			j.operands(instr)
			j.instruction(instr)
		}
	}
}

// operands checks the values an instruction uses and the value it makes.
//
// Rule: no value is a package-level variable, and none is a function unless
// the instruction calls it. No value has a floating-point or complex type, or
// the type unsafe.Pointer. This rejects more than arithmetic on such values,
// which keeps the rule to one line.
func (j *judge) operands(instr ssa.Instruction) {
	var callee *ssa.Value
	if call, ok := instr.(ssa.CallInstruction); ok {
		callee = &call.Common().Value
	}
	for _, operand := range instr.Operands(nil) {
		switch v := (*operand).(type) {
		case nil:
			continue
		case *ssa.Global:
			j.blame(ReasonVariable, j.at, v.Object(), "reference to a package-level variable", nil)
		case *ssa.Function:
			if operand != callee {
				j.reject(ReasonFuncValue, j.at, "function used as a value")
			}
		}
		j.valueType((*operand).Type())
	}
	if v, ok := instr.(ssa.Value); ok {
		j.valueType(v.Type())
	}
}

// valueType rejects the types no value of a comptime function may have.
func (j *judge) valueType(t types.Type) {
	basic, ok := t.Underlying().(*types.Basic)
	switch {
	case !ok:
	case basic.Info()&(types.IsFloat|types.IsComplex) != 0:
		j.reject(ReasonFloat, j.at, "value of type "+basic.Name())
	case basic.Kind() == types.UnsafePointer:
		j.reject(ReasonUnsafe, j.at, "unsafe.Pointer")
	}
}

// instruction checks one instruction against the allow-list.
//
// Rule: these instructions are allowed as they are. They read memory, name a
// part of it, allocate it, or move control, and each does the same on every
// target: Alloc, Phi, ChangeType, MakeInterface, FieldAddr, Field, IndexAddr,
// Index, Slice, Extract, Jump, If, Return, Panic. An index out of range, a nil
// pointer and an explicit panic are deterministic run-time panics.
//
// MakeInterface is here because panic and error results take an interface.
// A comptime function can make, move and return an interface, and compare it
// with nil. It cannot look inside: a type assertion, a method call and a
// comparison with anything but nil are all rejected.
//
// The other cases have a rule of their own. Any instruction not named here is
// rejected by the default case.
func (j *judge) instruction(instr ssa.Instruction) {
	switch i := instr.(type) {
	case *ssa.Alloc, *ssa.Phi, *ssa.ChangeType, *ssa.MakeInterface, *ssa.FieldAddr, *ssa.Field,
		*ssa.IndexAddr, *ssa.Index, *ssa.Slice, *ssa.Extract, *ssa.Jump, *ssa.If, *ssa.Return,
		*ssa.Panic:
	case *ssa.BinOp:
		j.binary(i)
	case *ssa.UnOp:
		j.unary(i)
	case *ssa.Convert:
		j.convert(i)
	case *ssa.Call:
		j.call(i.Common())
	case *ssa.Store:
		j.write(i.Addr, "store")
	case *ssa.MakeSlice:
		// Rule: make allocates memory the function owns. A length that is too
		// large panics, and how large that is depends on the size of int.
		j.need(SameIntSize)
	case *ssa.MakeMap:
		// Rule: make allocates a map the function owns. Its size is a hint,
		// and what a hint too large for the target does is left open.
		j.mapKey(i.Type())
		j.size(i.Reserve)
	case *ssa.Lookup:
		j.mapKey(i.X.Type())
	case *ssa.MapUpdate:
		j.mapKey(i.Map.Type())
		j.write(i.Map, "map update")
	case *ssa.Range:
		j.rangeOver(i.X.Type())
	case *ssa.Next:
		if !i.IsString {
			j.reject(ReasonMapRange, j.at, "range over a map")
		}
	case *ssa.Go, *ssa.Select, *ssa.Send, *ssa.MakeChan:
		j.reject(ReasonConcurrency, j.at, constructName(instr))
	case *ssa.MakeClosure:
		j.reject(ReasonFuncValue, j.at, "function literal")
	default:
		j.reject(ReasonUnsupported, j.at, constructName(instr))
	}
}

// constructName names the construct an instruction stands for.
func constructName(instr ssa.Instruction) string {
	switch instr.(type) {
	case *ssa.Go:
		return "go statement"
	case *ssa.Select:
		return "select statement"
	case *ssa.Send:
		return "channel send"
	case *ssa.MakeChan:
		return "channel creation"
	case *ssa.Defer, *ssa.RunDefers:
		return "defer statement"
	case *ssa.TypeAssert:
		return "type assertion"
	case *ssa.ChangeInterface:
		return "conversion between interface types"
	}
	return "instruction " + strings.TrimPrefix(fmt.Sprintf("%T", instr), "*ssa.")
}

// binary checks a binary operation.
//
// Rule: arithmetic, shifts and logic on integers, logic on booleans and
// concatenation of strings are allowed, and any other operand type is
// rejected. A division by zero is a deterministic panic. A result of type
// int, uint or uintptr wraps at the size of the target.
//
// Rule: a comparison is allowed between integers, booleans or strings, and of
// any value with nil. Other comparisons are rejected: whether two pointers to
// distinct zero-size variables are equal is unspecified, and an interface, a
// struct or an array can hold such pointers.
func (j *judge) binary(op *ssa.BinOp) {
	switch op.Op {
	case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
		if !isPlain(op.X.Type()) && !isNil(op.X) && !isNil(op.Y) {
			j.reject(ReasonUnsupported, j.at, "comparison of values that are not integers, booleans or strings")
		}
	case token.ADD:
		j.arithmetic(op.Type())
		if b, ok := op.Type().Underlying().(*types.Basic); ok && b.Info()&types.IsString != 0 {
			// A concatenation longer than the int of the target can hold panics there.
			j.need(SameIntSize)
		}
	case token.SUB, token.MUL, token.QUO, token.REM, token.AND, token.OR, token.XOR, token.SHL, token.SHR, token.AND_NOT:
		j.arithmetic(op.Type())
	default:
		j.reject(ReasonUnsupported, j.at, "operator "+op.Op.String())
	}
}

// arithmetic checks the result type of an operator that computes a value.
func (j *judge) arithmetic(t types.Type) {
	if !isPlain(t) {
		j.reject(ReasonUnsupported, j.at, "operator on values that are not integers, booleans or strings")
	}
	j.sized(t)
}

// unary checks a unary operation.
//
// Rule: a load through any pointer is allowed, since reading has no effect;
// so are negation and complement of integers and the negation of a boolean.
// A channel receive is not.
func (j *judge) unary(op *ssa.UnOp) {
	switch op.Op {
	case token.ARROW:
		j.reject(ReasonConcurrency, j.at, "channel receive")
	case token.MUL:
	case token.SUB, token.XOR, token.NOT:
		j.arithmetic(op.Type())
	default:
		j.reject(ReasonUnsupported, j.at, "operator "+op.Op.String())
	}
}

// convert checks a conversion that changes the representation of a value.
//
// Rule: conversions among integers and strings, and from a slice of bytes or
// runes to a string, are allowed. A conversion from a string to a slice is
// not: the capacity of the result is unspecified, and gc gives a different
// one when the slice does not escape.
func (j *judge) convert(c *ssa.Convert) {
	_, toSlice := c.Type().Underlying().(*types.Slice)
	_, fromSlice := c.X.Type().Underlying().(*types.Slice)
	switch {
	case toSlice:
		j.reject(ReasonUnspecified, j.at, "conversion of a string to a slice, whose capacity is unspecified")
	case isPlain(c.Type()) && (fromSlice || isPlain(c.X.Type())):
		j.sized(c.Type())
	default:
		j.reject(ReasonUnsupported, j.at, "conversion that is not among integers and strings")
	}
}

// size asks for the condition when a size given to make is not a constant.
func (j *judge) size(size ssa.Value) {
	if _, constant := size.(*ssa.Const); size != nil && !constant {
		j.need(SameIntSize)
	}
}

// sized records that a result of type int, uint or uintptr depends on the size of int.
//
// Rule: the condition is asked of an operator or a conversion that makes such
// a value, of make, and of a constant that needs more than 32 bits. It is not
// asked of a function that only moves, compares or indexes with such values.
// The justification: an argument is the same number on host and target by
// assumption, and moving, comparing and widening a number give the same
// result at any size.
func (j *judge) sized(t types.Type) {
	if sizedByTarget(t) {
		j.need(SameIntSize)
	}
}

// isPlain reports whether a type is a boolean, an integer or a string underneath.
func isPlain(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&(types.IsBoolean|types.IsInteger|types.IsString) != 0
}

// isNil reports whether a value is the constant nil.
func isNil(v ssa.Value) bool {
	c, ok := v.(*ssa.Const)
	return ok && c.IsNil()
}

// mapKey checks the key type of a map that is made, read or updated.
//
// Rule: a map is allowed when its keys are integers, booleans or strings,
// whose equality is specified. A lookup in a string is an index and is allowed.
func (j *judge) mapKey(t types.Type) {
	if m, ok := t.Underlying().(*types.Map); ok && !isPlain(m.Key()) {
		j.reject(ReasonUnsupported, j.at, "map with a key that is not an integer, a boolean or a string")
	}
}

// rangeOver checks the operand of a range loop that needs an iterator.
//
// Rule: ranging over a string is allowed. The order of a range over a map is
// unspecified, so that is not.
func (j *judge) rangeOver(t types.Type) {
	if _, ok := t.Underlying().(*types.Map); ok {
		j.reject(ReasonMapRange, j.at, "range over a map")
	}
}

// write checks that the target of a write is memory the function allocated.
func (j *judge) write(target ssa.Value, what string) {
	if !owned(target, map[*ssa.Phi]bool{}) {
		j.reject(ReasonForeignWrite, j.at, what+" to memory the function did not allocate")
	}
}

// owned reports whether a pointer, slice or map refers only to memory this function allocated.
//
// Rule: a function owns what new, make, a composite literal or a local
// variable allocates in it, and the fields, elements and subslices of that.
// It owns nothing it loaded from memory, received as a parameter or got from
// a call, even when that happens to be its own.
func owned(v ssa.Value, seen map[*ssa.Phi]bool) bool {
	switch v := v.(type) {
	case *ssa.Alloc, *ssa.MakeSlice, *ssa.MakeMap:
		return true
	case *ssa.FieldAddr:
		return owned(v.X, seen)
	case *ssa.IndexAddr:
		return owned(v.X, seen)
	case *ssa.Slice:
		return owned(v.X, seen)
	case *ssa.ChangeType:
		return owned(v.X, seen)
	case *ssa.Phi:
		return ownedPhi(v, seen)
	}
	return false
}

// ownedPhi reports whether every value that can reach a phi is owned.
func ownedPhi(phi *ssa.Phi, seen map[*ssa.Phi]bool) bool {
	if seen[phi] {
		// A phi that reaches itself adds no value of its own.
		return true
	}
	seen[phi] = true
	for _, edge := range phi.Edges {
		if !owned(edge, seen) {
			return false
		}
	}
	return true
}

// call checks a call.
//
// Rule: a call is allowed when it names its callee: a built-in on the list of
// [judge.builtin], or a declared function or method that is comptime itself.
// A call through an interface or a function value is not.
func (j *judge) call(c *ssa.CallCommon) {
	if c.IsInvoke() {
		j.reject(ReasonDynamicCall, j.at, "call through an interface")
		return
	}
	switch callee := c.Value.(type) {
	case *ssa.Builtin:
		j.builtin(c, callee.Name())
	case *ssa.Function:
		j.static(callee)
	default:
		j.reject(ReasonDynamicCall, j.at, "call through a function value")
	}
}

// static checks a call of a named function or method.
func (j *judge) static(callee *ssa.Function) {
	fn, _ := callee.Object().(*types.Func)
	switch {
	case fn == nil:
		j.reject(ReasonUnsupported, j.at, "call of a function without a declaration")
	case len(callee.TypeArgs()) > 0 || callee.TypeParams().Len() > 0:
		j.blame(ReasonGeneric, j.at, fn, "call of a generic function", nil)
	case callee.Synthetic != "" && !strings.HasPrefix(callee.Synthetic, "from type information"):
		j.blame(ReasonUnsupported, j.at, fn, "call of a synthetic function", nil)
	default:
		j.depend(fn, j.at, "call")
	}
}

// builtin checks a call of a built-in function.
//
// Rule: len, cap, min and max compute a value; copy, delete and clear write,
// and are allowed on memory the function owns. append is not allowed: the
// capacity of its result is unspecified, and a later append, slice expression
// or cap can observe it. Any other built-in is rejected by name.
func (j *judge) builtin(c *ssa.CallCommon, name string) {
	switch name {
	case "len", "cap":
		if _, ok := c.Args[0].Type().Underlying().(*types.Chan); ok {
			j.reject(ReasonConcurrency, j.at, name+" of a channel")
		}
	case "min", "max":
	case "copy", "delete", "clear":
		j.mapKey(c.Args[0].Type())
		j.write(c.Args[0], name)
	case "append":
		j.reject(ReasonUnspecified, j.at, "append, whose result has an unspecified capacity")
	case "recover":
		j.reject(ReasonRecover, j.at, "recover")
	case "close":
		j.reject(ReasonConcurrency, j.at, "close of a channel")
	default:
		j.reject(ReasonUnsupported, j.at, "built-in "+name)
	}
}

# sumtype

sumtype reads a package and writes the boilerplate for its sum types.

    go run ./examples/sumtype/cmd/sumtype ./path/to/package

A directive in the doc comment of a type declaration indicates the sum type and
its default variant. The type declarations on the lines right after the
directive are treated as its variants.

    //sumtype: Shape, default=Circle
    type Circle struct{ R float64 }
    type Rect struct{ W, H float64 }

For `shapes.go` the tool writes `shapes_sumtype.go` in the same directory:

    type Shape struct{ v shapeVariant }

    func NewShape[V shapeOf](v V) Shape
    func (s Shape) Match[R any](onCircle func(Circle) R, onRect func(Rect) R) R

`Match` is a generic method, so the generated package needs Go 1.27.

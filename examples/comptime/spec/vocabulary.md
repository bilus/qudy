# Vocabulary

One term per concept. Go's word wins when it exists. We borrow a term only
where Go has none and the term brings a pattern with it.

## How the main words fit together

- **Comptime** is a property. A symbol has it or lacks it.
- A **fact** states that one symbol is comptime. There is one property, and
  there are many facts.
- A fact has one of two origins. A person **asserts** a **trusted** fact, for
  example by tagging a declaration. The analyzer **infers** an **inferred**
  fact from other facts, by a comptime rule.
- A **directive** is the comment line `//qudy:comptime`. A declaration that it
  precedes is **tagged**.
- **Run-time** is the default binding time. It means that no fact says
  comptime, and not that anyone showed the symbol to be run-time.
- We **prove**, in the proof design, that each comptime rule is sound. The analyzer
  proves nothing; it infers.

## Defined by this specification

| Term | Meaning |
| --- | --- |
| **tagged** | Said of a declaration: a `//qudy:comptime` directive immediately precedes it. |
| **build time** | While `go-comptime` runs, before it starts the `go` command. |
| **`go-comptime`**, the **driver** | The command that wraps the `go` command. |
| **comptime file** | The copy of a source file in which literals replace the tagged initialization expressions. |
| **template** | The qudy template that `go-comptime` writes for one source file. |
| **generator** | The Go program that qudy compiles from a template. |
| **comptime rule** | Says when one kind of expression or statement is comptime. A derivation is a chain of comptime rules and facts. |
| **comptime type** | A type that a value in comptime code may have. `comptime-rules.md` holds the list. Zig's "comptime-only type" means the opposite: a type that exists only at compile time. |
| **test-program generator** | The program in `examples/comptime/gen` that writes Go programs to test the analyzer. It is never called "the generator". |

## Go's own words

| Term | Meaning here | From |
| --- | --- | --- |
| **constant** | Only what the Go specification calls a constant. A comptime value is never called a constant: some constants depend on the target, and constant arithmetic differs from run-time arithmetic. | Go spec, Constants |
| **compile time**, **run time**, **run-time** | The Go specification's two times. Comptime is not compile time: `go-comptime` evaluates before the compiler starts. | Go spec |
| **directive** | A comment line that a tool reads, such as `//line` or `//go:generate`. This specification defines one, `//qudy:comptime`. | `go/ast`; `go doc cmd/compile` |
| **symbol** | A package-level function, variable or constant, or a method. A fact is about one symbol. | `go doc` |
| **source file** | A file the user wrote, as opposed to a comptime file. | Go spec, Source file organization |
| **package-level variable**, **initialization expression** | A variable declared outside every function, and the expression that gives it its first value. Not "global", which the Go specification never uses. | Go spec, Package initialization |
| **exported**, **not exported** | The boundary of what importers can name. | Go spec, Exported identifiers |
| **self-contained** | A value that carries all its data: predeclared types, arrays and structs of those. It has no identity, so it can be copied and shared freely. | Go spec, Representation of values |
| **reference**, **underlying data** | What a pointer, slice or map value contains instead. | Go spec, same section |
| **new variable** | What `new`, `make`, a composite literal or a declaration creates. | Go spec, Allocation |
| **addressable**, **assignable** | The two ways a variable becomes writable by other code. | Go spec |
| **static type**, **dynamic type** | The declared type and the type of the stored value. This use of "static" and "dynamic" is why binding times need other words. | Go spec, Variables |
| **static call**, **dynamic call** | Whether the callee is named. A call through an interface or a function value is dynamic. | `go/ssa`, x/tools |
| **run-time panic** | Go's failure. It is a value with a dynamic type, and can be recovered. | Go spec, Run-time panics |
| **order of evaluation**, **unspecified**, **implementation-specific** | Where the Go specification leaves a choice open. | Go spec |
| **architecture-independent** | The Go specification's word for sizes that do not vary with the target. Extended here from types to values and functions: the same result for every target. | Go spec, Numeric types |
| **target**, **host** | The machine the program runs on, and the one the tools run on. | go.dev/doc/install/source |
| **build constraint**, **build configuration** | Which files make up a package; and the whole tuple of `GOOS`, `GOARCH`, levels such as `GOAMD64`, tags and toolchain release. | go.dev/ref/mod glossary, `go/packages` |
| **overlay** | The file given to `-overlay`, which makes a build read one file in place of another. | `go help build` |
| **complete program**, **main package**, **internal package** | The unit in which "no importer assigns it" can be proven, and the two ways Go bounds the importers. | Go spec; `go help importpath` |
| **fact** | A statement that one symbol is comptime. Every fact is trusted or inferred. A package hands its facts to its importers. | `go/analysis` |
| **analyzer**, **pass** | The program that infers facts, and one run of it over one package. Its report for one symbol is a **verdict**: a fact, or the reasons why it inferred none. | `go/analysis` |
| **sound**, **precise** | Sound: the analyzer infers no false fact, provided that every trusted fact is true. Precise: it infers a fact for the symbols that are comptime. | `go/callgraph` |
| **effect**, **side effect** | A change to state that existed before. The Go specification uses "side effects" twice and never defines the term. | Go spec; x/tools inliner |
| **duplicable** | Evaluating twice equals evaluating once and sharing the result. | x/tools inliner |
| **single assignment** | Assigned only by its declaration. The inliner defines it for local variables; here it extends to package-level variables. | x/tools inliner |
| **hazard** | A construct that can make a value run-time. An **impurity** is a hazard that the test-program generator plants in a program. | x/tools inliner, where it names an order-of-evaluation hazard |

## Borrowed, each with its pattern

| Term | Meaning here | From | Pattern it brings |
| --- | --- | --- | --- |
| **comptime** | The property that facts are about. A value has it when nothing that exists only at run time can change the value, and when computing the value leaves nothing behind. A symbol has it when its value does. | Zig | A name with no other meaning in Go. |
| **binding time** | When a value can be computed. It has two values, comptime and run-time. Run-time is the default: it means that no fact says comptime. | Jones, Gomard, Sestoft | A **binding-time signature** such as comptime → comptime is defined by induction on the Go type. It is what a fact carries. |
| **binding-time analysis** | The work of the analyzer: it infers facts, and from the facts it decides whether an expression is comptime or run-time. It names the first section of the requirements. | Jones, Gomard, Sestoft | |
| **derivation** | The chain of comptime rules and facts that yields a fact. The analyzer looks for one for a trusted fact too. Without one, the fact rests on the person's word alone. | proof systems | It answers "why is this symbol comptime?", so `go-comptime` can print it for an audit. |
| **partially comptime** | A value with a comptime part and a run-time part. | partial evaluation, "partially static" | The binding time of a struct is a struct of binding times. |
| **as-if rule** | Same observable behaviors with and without the rewrite. | C++ | Names the governing rule. Go has no word for it. |
| **side-effect free**, **independent**, **deterministic** | Three of the four parts of comptime. | Sălcianu and Rinard; C23; Finifter | The decomposition comptime = side-effect free + independent + deterministic + architecture-independent, after DetTrace's reproducible = deterministic + portable. |
| **fresh** | A new variable that nothing else can reach yet. | JPure, Barnett | Writes to fresh variables are masked, as in Haskell's `runST`. The Go specification has "new variable" and no adjective for "not yet shared". |
| **thunkable**, **copyable**, **discardable**, **central** | Behaves like a value; may be evaluated twice; may be dropped; commutes with anything. | Führmann | Thunkable implies the other three. A panic is copyable and neither discardable nor central. Each property has a rewrite, **Copy**, **Discard**, **Swap**, **Hoist**, and each rewrite is a test. |
| **total comptime** | Comptime, never panics, always terminates. | Koka: `total` against `pure` | Decides which rewrites are legal: Hoist and Discard need total. |
| **comptime relative to a parameter** | Comptime when the function or interface argument is. | effect polymorphism, Rytz and Odersky | The function case of the definition, applied to a parameter. |
| **inferred**, **trusted** | The two origins of a fact. The analyzer infers a fact by a comptime rule. A person asserts a trusted fact, which holds whether or not the analyzer finds a derivation for it. | Safe Haskell | Inference never yields a trusted fact. The symbols in the index of the test-program generator are trusted. |
| **lift**, **liftable** | Write a comptime value as Go source; a type whose values all can be. The result is a literal. | Jones, Gomard, Sestoft | |
| **residual** | What the generator writes for run time. A comptime file is a residual program. | same | |
| **suspend** | Leave an expression for run time. | Andersen | Reads well with "opportunistic". |
| **evaluation budget** | The limit on steps, depth and memory of one evaluation. | Zig's branch quota | |
| **soundy** | Sound except for named, published assumptions. | Livshits and others | |

## Words to avoid

| Word | Why | Say instead |
| --- | --- | --- |
| **tag** as a noun | Go has build tags and struct tags. | directive for the line, tagged for the declaration |
| **assertion** | Go's type assertion is checked at run time. A trusted fact holds on a person's word. | "a person asserts"; trusted fact |
| **allow-list** | It names a list. A comptime rule also states a condition. | comptime rules |
| **prove**, **prover** for the analyzer | We prove that the comptime rules are sound, and the Go compiler has a pass named `prove`. | infer, analyzer |
| **property** for a fact | Comptime is the property. A fact states that one symbol has it. | fact |
| **pure** | Six incompatible meanings in the literature. In Go, the inliner means "reads only" and staticcheck lists `time.Now` as pure. | the part that fails, or comptime |
| **static**, **dynamic** as binding times | Taken by static and dynamic type, and by static and dynamic call. | comptime, run-time |
| **constant** for a comptime value | Taken by the Go specification. | comptime value |
| **portable**, **build context**, **confined**, **write-once**, **never-written** | Synonyms from the reports for terms Go already has. | architecture-independent, build configuration, side-effect free, single assignment |
| **division**, **congruent** | The literature's names for the list and its soundness. Go's tools say it plainly. | the list, sound |
| **level**, **phase**, **promotion** | Each means two unrelated things across Haskell, Rust, Racket and the Go compiler. | binding time; lift |

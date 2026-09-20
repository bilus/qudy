package shapes

//sumtype: Shape, default=Circle
type Circle struct{ R float64 }
type Rect struct{ W, H float64 }

// Point follows a blank line, so it is not a variant.
type Point struct{ X, Y float64 }

// sumtype: token, default=word
type (
	word   string
	number int
)

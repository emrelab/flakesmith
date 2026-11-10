package appglobals

type registry struct{ value int }

var state registry

func (r *registry) Pollute()   { r.value = 1 }
func (r *registry) Value() int { return r.value }

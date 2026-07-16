package sonolus

func Sign(x float64) float64              { return 0 }
func Frac(x float64) float64              { return 0 }
func Clamp(x, min, max float64) float64   { return 0 }
func Lerp(a, b, t float64) float64        { return 0 }
func LerpClamped(a, b, t float64) float64 { return 0 }
func Unlerp(a, b, x float64) float64      { return 0 }
func UnlerpClamped(a, b, x float64) float64 {
	return 0
}
func Remap(a, b, c, d, x float64) float64 { return 0 }
func RemapClamped(a, b, c, d, x float64) float64 {
	return 0
}

type EaseDirection string
type EaseCurve string

const (
	EaseIn      EaseDirection = "In"
	EaseOut     EaseDirection = "Out"
	EaseInOut   EaseDirection = "InOut"
	EaseOutIn   EaseDirection = "OutIn"
	EaseSine    EaseCurve     = "Sine"
	EaseQuad    EaseCurve     = "Quad"
	EaseCubic   EaseCurve     = "Cubic"
	EaseQuart   EaseCurve     = "Quart"
	EaseQuint   EaseCurve     = "Quint"
	EaseExpo    EaseCurve     = "Expo"
	EaseCirc    EaseCurve     = "Circ"
	EaseBack    EaseCurve     = "Back"
	EaseElastic EaseCurve     = "Elastic"
)

func Ease(direction EaseDirection, curve EaseCurve, value float64) float64 { return 0 }
func Linstep(value float64) float64                                        { return 0 }
func Smoothstep(value float64) float64                                     { return 0 }
func Smootherstep(value float64) float64                                   { return 0 }
func StepStart(value float64) float64                                      { return 0 }
func StepEnd(value float64) float64                                        { return 0 }

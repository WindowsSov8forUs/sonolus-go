package model

type Number struct {
	Digits   [2]float64
	Exponent int
}

func (n *Number) Copy(source *Number) {
	for i := 0; i < len(n.Digits); i++ {
		n.Digits[i] = source.Digits[i]
	}
	n.Exponent = source.Exponent
}

type State struct {
	Left, Right, Result, A, B, Tail Number
}

func (s *State) Init() {
	s.Left = Number{[2]float64{11, 12}, 13}
	s.Right = Number{[2]float64{21, 22}, 23}
	s.Result = Number{[2]float64{31, 32}, 33}
	s.A = Number{[2]float64{41, 42}, 43}
	s.B = Number{[2]float64{51, 52}, 53}
	s.Tail = Number{[2]float64{61, 62}, 63}
}

func (s *State) CopyPair(out, a, b *Number) {
	s.A.Copy(a)
	s.B.Copy(b)
	out.Copy(&s.B)
}

func (s *State) Exercise() {
	s.CopyPair(&s.Result, &s.Left, &s.Right)
	s.CopyPair(&s.Left, &s.Result, &s.Right)
}

func (s *State) Values() [18]float64 {
	return [18]float64{
		s.Left.Digits[0], s.Left.Digits[1], float64(s.Left.Exponent),
		s.Right.Digits[0], s.Right.Digits[1], float64(s.Right.Exponent),
		s.Result.Digits[0], s.Result.Digits[1], float64(s.Result.Exponent),
		s.A.Digits[0], s.A.Digits[1], float64(s.A.Exponent),
		s.B.Digits[0], s.B.Digits[1], float64(s.B.Exponent),
		s.Tail.Digits[0], s.Tail.Digits[1], float64(s.Tail.Exponent),
	}
}

func (s *State) Observe() [18]float64 {
	s.Exercise()
	return s.Values()
}

func (s *State) ReadDirect() [3]float64 {
	s.CopyPair(&s.Result, &s.Left, &s.Right)
	s.CopyPair(&s.Left, &s.Result, &s.Right)
	s.A.Copy(&s.Right)
	return [3]float64{s.B.Digits[0], s.B.Digits[1], float64(s.B.Exponent)}
}

func (s *State) Once() {
	s.CopyPair(&s.Result, &s.Left, &s.Right)
}

func (s *State) Branch(enter bool) {
	if enter {
		for repeat := 0; repeat < 2; repeat++ {
			s.Exercise()
		}
	}
}

type Pair [2]State

func (p *Pair) Init() {
	p[0].Init()
	p[1].Init()
	p[1].Left = Number{[2]float64{101, 102}, 103}
	p[1].Right = Number{[2]float64{111, 112}, 113}
}

func nextIndex(index *int) int {
	result := *index
	*index = 1 - *index
	return result
}

func (p *Pair) NumberAt(index int) *Number {
	return &p[index].Result
}

func (n *Number) Rebind(other *Number) {
	alias := &n.Exponent
	n = other
	*alias += 1
	n.Digits[0] = 81
}

func (p *Pair) Alias(index int, enter bool) [37]float64 {
	first := &p[nextIndex(&index)].Left
	second := p.NumberAt(1 - index)
	first.Exponent = 71
	second.Digits[1] = 72
	if enter {
		first.Copy(&p[index].Right)
	}
	for repeat := 0; repeat < 2; repeat++ {
		second.Exponent += 1
	}
	first.Rebind(second)
	var result [37]float64
	for parent := 0; parent < 2; parent++ {
		for slot, value := range p[parent].Values() {
			result[parent*18+slot] = value
		}
	}
	result[36] = float64(index)
	return result
}

func Conversion(value float64) [2]float64 {
	integer := int(value)
	return [2]float64{value, float64(integer)}
}

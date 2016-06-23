package jval

import (
	"math"
	"regexp"
	"strconv"
)

type Error struct {
	Label   string      `json:"label"`
	Field   []string    `json:"field"`
	Context interface{} `json:"context"`
}

// Error implements native "error" interface
func (e Error) Error() string {
	return e.Label
}

func (e Error) Equals(o Error) bool {
	if len(e.Field) != len(o.Field) {
		return false
	}
	if e.Label != o.Label {
		return false
	}
	for i, l := 0, len(e.Field); i < l; i++ {
		if e.Field[i] != o.Field[i] {
			return false
		}
	}
	return e.Context == o.Context
}

var NoErrors = []Error{}

type Validator interface {
	Validate(value interface{}, field []string) []Error
	Traverse(interface{}, func(interface{}, Validator))
}

type Lambda func(v interface{}, f []string) []Error

func (l Lambda) Validate(v interface{}, f []string) []Error {
	return l(v, f)
}

func (l Lambda) Traverse(v interface{}, f func(interface{}, Validator)) {
	f(v, l)
}

type AnythingValidator struct{}

func Anything() Validator {
	return AnythingValidator{}
}

func (a AnythingValidator) Validate(v interface{}, f []string) []Error {
	return NoErrors
}

func (a AnythingValidator) Traverse(v interface{}, f func(interface{}, Validator)) {
	f(v, a)
}

type StringValidator struct{}

func String() Validator {
	return StringValidator{}
}

func (a StringValidator) Validate(v interface{}, f []string) []Error {
	if _, k := v.(string); k {
		return NoErrors
	}
	return []Error{Error{"value_must_be_string", f, nil}}
}

func (a StringValidator) Traverse(v interface{}, f func(interface{}, Validator)) {
	f(v, a)
}

type NumberValidator struct{}

func Number() Validator {
	return NumberValidator{}
}

func (a NumberValidator) Validate(v interface{}, f []string) []Error {
	if _, k := v.(float64); k {
		return NoErrors
	}
	return []Error{Error{"value_must_be_number", f, nil}}
}

func (a NumberValidator) Traverse(v interface{}, f func(interface{}, Validator)) {
	f(v, a)
}

type BooleanValidator struct{}

func Boolean() Validator {
	return BooleanValidator{}
}

func (a BooleanValidator) Validate(v interface{}, f []string) []Error {
	if _, k := v.(bool); k {
		return NoErrors
	}
	return []Error{Error{"value_must_be_boolean", f, nil}}
}

func (a BooleanValidator) Traverse(v interface{}, f func(interface{}, Validator)) {
	f(v, a)
}

type NullValidator struct{}

func Null() Validator {
	return NullValidator{}
}

func (a NullValidator) Validate(v interface{}, f []string) []Error {
	if v == nil {
		return NoErrors
	}
	return []Error{Error{"value_must_be_null", f, nil}}
}

func (a NullValidator) Traverse(v interface{}, f func(interface{}, Validator)) {
	f(v, a)
}

type AndValidator []Validator

func And(vs ...Validator) Validator {
	if len(vs) == 0 {
		panic("and of 0 conditions")
	}
	if len(vs) == 1 {
		return vs[0]
	}
	return AndValidator(vs)
}

func (a AndValidator) Validate(v interface{}, f []string) []Error {
	ae := make([]Error, 0, len(a))
	for _, b := range a {
		es := b.Validate(v, f)
		for _, e := range es {
			if e.Label == "and" {
				ae = append(ae, e.Context.([]Error)...)
			} else {
				ae = append(ae, e)
			}
		}
	}
	if len(ae) == 0 {
		return NoErrors
	}
	ue := uniqueErrors(ae)
	if len(ue) == 1 {
		return ue
	}
	return []Error{Error{"and", []string{}, ue}}
}
func (a AndValidator) Validators() []Validator {
	return []Validator(a)
}

func (a AndValidator) Traverse(v interface{}, f func(interface{}, Validator)) {
	for _, b := range a {
		b.Traverse(v, f)
	}
}

// type TupleValidator []Validator

// func Tuple(vs ...Validator) Validator {
// 	return TupleValidator(vs)
// }

// func (a TupleValidator) Validate(v interface{}, f []string) []Error {
// 	return And(Array(Anything()), Length(len(a)), Lambda(func(v interface{}, f []string) []Error {
// 		for i, b := range a {
// 			u, _ := v.ArrayIndex(i)
// 			es := b.Validate(u, append(f, strconv.Itoa(i)))
// 			if len(es) != 0 {
// 				return es
// 			}
// 		}
// 		return NoErrors
// 	})).Validate(v, f)
// }
// func (a TupleValidator) Validators() []Validator {
// 	return []Validator(a)
// }

type OrValidator []Validator

func Or(vs ...Validator) Validator {
	if len(vs) == 0 {
		panic("or of 0 conditions")
	}
	if len(vs) == 1 {
		return vs[0]
	}
	return OrValidator(vs)
}

func (b OrValidator) Validate(v interface{}, f []string) []Error {
	ae := make([]Error, 0, len(b))
	for _, a := range b {
		es := a.Validate(v, f)
		if len(es) == 0 {
			return NoErrors
		}
		for _, e := range es {
			if e.Label == "or" {
				ae = append(ae, e.Context.([]Error)...)
			} else {
				ae = append(ae, e)
			}
		}
	}
	if len(ae) == 0 {
		return NoErrors
	}
	ue := uniqueErrors(ae)
	if len(ue) == 1 {
		return ue
	}
	return []Error{Error{"or", []string{}, ue}}
}

func (a OrValidator) Validators() []Validator {
	return []Validator(a)
}

func (a OrValidator) Traverse(v interface{}, f func(interface{}, Validator)) {
	for _, b := range a {
		b.Traverse(v, f)
	}
}

type CaseValidator map[string]Validator

func Case(d map[string]Validator) Validator {
	return CaseValidator(d)
}

func (d CaseValidator) Validate(v interface{}, f []string) []Error {
	o, k := v.(map[string]interface{})
	if !k {
		return []Error{Error{"value_must_be_object", f, nil}}
	}
	if len(o) != 1 {
		return []Error{Error{"object_must_have_exactly_one_key", f, nil}}
	}
	c := ""
	for k, _ := range o {
		c = k
	}
	vd, k := d[c]
	if !k {
		return []Error{Error{"case_not_defined", f, c}}
	}
	tv := o[c]
	return vd.Validate(tv, append(f, c))
}

func (a CaseValidator) Structure() map[string]Validator {
	return (map[string]Validator)(a)
}

func (a CaseValidator) Traverse(v interface{}, f func(interface{}, Validator)) {
	c, o := "", v.(map[string]interface{})
	for k, _ := range o {
		c = k
	}
	a[c].Traverse(o[c], f)
}

type ObjectValidator map[string]Validator

func Object(d map[string]Validator) Validator {
	return ObjectValidator(d)
}

func (d ObjectValidator) Validate(v interface{}, f []string) []Error {
	o, k := v.(map[string]interface{})
	if !k {
		return []Error{Error{"value_must_be_object", f, nil}}
	}
	ae := make([]Error, 0, len(d))
	for k, _ := range o {
		if _, ok := d[k]; !ok {
			ae = append(ae, Error{"unexpected_object_key", f, k})
		}
	}
	for k, a := range d {
		u, x := o[k]
		if !x {
			ae = append(ae, Error{"missing_object_key", f, k})
			continue
		}
		ae = append(ae, a.Validate(u, append(f, k))...)
	}
	return ae
}

func (a ObjectValidator) Structure() map[string]Validator {
	return (map[string]Validator)(a)
}

func (a ObjectValidator) Traverse(v interface{}, f func(interface{}, Validator)) {
	for k, v := range v.(map[string]interface{}) {
		a[k].Traverse(v, f)
	}
}

type MapValidator struct {
	e Validator
}

func Map(e Validator) Validator {
	return MapValidator{e}
}

func (a MapValidator) Validate(v interface{}, f []string) []Error {
	o, k := v.(map[string]interface{})
	if !k {
		return []Error{Error{"value_must_be_object", f, nil}}
	}
	ae := make([]Error, 0, 8)
	for k, u := range o {
		ae = append(ae, a.e.Validate(u, append(f, k))...)
	}
	return ae
}
func (a MapValidator) Validator() Validator {
	return a.e
}

func (a MapValidator) Traverse(v interface{}, f func(interface{}, Validator)) {
	for _, v := range v.(map[string]interface{}) {
		a.e.Traverse(v, f)
	}
}

type ArrayValidator struct {
	e Validator
}

func Array(e Validator) Validator {
	return ArrayValidator{e}
}

func (a ArrayValidator) Validate(v interface{}, f []string) []Error {
	o, k := v.([]interface{})
	if !k {
		return []Error{Error{"value_must_be_array", f, nil}}
	}
	ae := make([]Error, 0, 8)
	for i, u := range o {
		ae = append(ae, a.e.Validate(u, append(f, strconv.Itoa(i)))...)
	}
	return ae
}
func (a ArrayValidator) Validator() Validator {
	return a.e
}

func (a ArrayValidator) Traverse(v interface{}, f func(interface{}, Validator)) {
	for _, v := range v.([]interface{}) {
		a.e.Traverse(v, f)
	}
}

type RegexValidator struct {
	x, l string
	i, m bool
}

// see https://golang.org/pkg/regexp/syntax/
func Regex(x, l string, i, m bool) Validator {
	return RegexValidator{x, l, i, m}
}

func (a RegexValidator) Label() string {
	return a.l
}

func (a RegexValidator) Expression() string {
	return a.x
}

func (a RegexValidator) Modifiers() (i, m bool) {
	return a.i, a.m
}

func (a RegexValidator) Regex() *regexp.Regexp {
	x := a.x
	if a.i {
		x = `(?i)` + x
	}
	if a.m {
		x = `(?m)` + x
	}
	return regexp.MustCompile(x)
}

func (a RegexValidator) Validate(v interface{}, f []string) []Error {
	return And(String(), Lambda(func(v interface{}, f []string) []Error {
		s := v.(string)
		if a.Regex().MatchString(s) {
			return NoErrors
		}
		return []Error{
			Error{a.l, f, map[string]interface{}{
				"regex": map[string]interface{}{
					"expression": a.x,
					"modifiers": map[string]bool{
						"i": a.i,
						"m": a.m,
					},
				},
			}},
		}
	})).Validate(v, f)
}

func (a RegexValidator) Traverse(v interface{}, f func(interface{}, Validator)) {
	f(v, a)
}

// type RegexStringValidator struct{}

// func RegexString() Validator {
// 	return RegexStringValidator{}
// }

// func (a RegexStringValidator) Validate(v interface{}, f []string) []Error {
// 	return And(String(), Lambda(func(v interface{}, f []string) []Error {
// 		s, _ := v.String()
// 		_, e := regexp.Compile(s)
// 		if e != nil {
// 			return []Error{Error{"value_must_be_regex_string", f, nil}}
// 		}
// 		return NoErrors
// 	})).Validate(v, f)
// }

type OptionalValidator struct {
	e Validator
}

func Optional(e Validator) Validator {
	return OptionalValidator{e}
}

func (a OptionalValidator) Validate(v interface{}, f []string) []Error {
	if v == nil {
		return NoErrors
	}
	return a.e.Validate(v, f)
}
func (a OptionalValidator) Validator() Validator {
	return a.e
}

func (a OptionalValidator) Traverse(v interface{}, f func(interface{}, Validator)) {
	a.e.Traverse(v, f)
}

type LengthBetweenValidator struct {
	x, y int
}

func LengthBetween(x, y int) Validator {
	if y < x {
		panic("LengthBetween: y < x")
	}
	return LengthBetweenValidator{x, y}
}

func (a LengthBetweenValidator) Validate(v interface{}, f []string) []Error {
	return And(Or(String(), Array(Anything())), Lambda(func(v interface{}, f []string) []Error {
		l := -1
		switch t := v.(type) {
		case string:
			l = len(t)
		case []interface{}:
			l = len(t)
		}
		if l < a.x || l > a.y {
			if a.x == a.y {
				return []Error{
					Error{"value_must_have_length", f, a.x},
				}
			}
			return []Error{
				Error{"value_must_have_length_between", f, map[string]int{"min": a.x, "max": a.y}},
			}
		}
		return NoErrors
	})).Validate(v, f)
}

func (a LengthBetweenValidator) Min() int {
	return a.x
}

func (a LengthBetweenValidator) Max() int {
	return a.y
}

func (a LengthBetweenValidator) Traverse(v interface{}, f func(interface{}, Validator)) {
	f(v, a)
}

func Length(x int) Validator {
	return LengthBetween(x, x)
}

type NumberBetweenValidator struct {
	x, y float64
}

func NumberBetween(x, y float64) Validator {
	if y < x {
		panic("NumberBetween: y < x")
	}
	return NumberBetweenValidator{x, y}
}

func (a NumberBetweenValidator) Validate(v interface{}, f []string) []Error {
	return And(Number(), Lambda(func(v interface{}, f []string) []Error {
		l := v.(float64)
		if l < a.x || l > a.y {
			return []Error{
				Error{"value_must_have_value_between", f, map[string]float64{"min": a.x, "max": a.y}},
			}
		}
		return NoErrors
	})).Validate(v, f)
}

func (a NumberBetweenValidator) Traverse(v interface{}, f func(interface{}, Validator)) {
	f(v, a)
}

type WholeNumberValidator struct{}

func WholeNumber() Validator {
	return WholeNumberValidator{}
}

func (a WholeNumberValidator) Validate(v interface{}, f []string) []Error {
	return And(Number(), Lambda(func(v interface{}, f []string) []Error {
		n := v.(float64)
		_, r := math.Modf(n)
		if r != 0 {
			return []Error{Error{"value_must_be_whole_number", f, nil}}
		}
		return NoErrors
	})).Validate(v, f)
}

func (a WholeNumberValidator) Traverse(v interface{}, f func(interface{}, Validator)) {
	f(v, a)
}

type WholeNumberBetweenValidator struct {
	x, y int
}

func WholeNumberBetween(x, y int) Validator {
	if y < x {
		panic("WholeNumberBetween: y < x")
	}
	return WholeNumberBetweenValidator{x, y}
}

func (a WholeNumberBetweenValidator) Validate(v interface{}, f []string) []Error {
	return And(WholeNumber(), NumberBetween(float64(a.x), float64(a.y))).Validate(v, f)
}

func (a WholeNumberBetweenValidator) Traverse(v interface{}, f func(interface{}, Validator)) {
	f(v, a)
}

type ExactlyValidator struct {
	j interface{}
}

func Exactly(j interface{}) Validator {
	return ExactlyValidator{j}
}

func (a ExactlyValidator) Value() interface{} {
	return a.j
}

func (a ExactlyValidator) Validate(v interface{}, f []string) []Error {
	if v != a.j {
		return []Error{Error{"value_not_matched_exactly", f, nil}}
	}
	return NoErrors
}

func (a ExactlyValidator) Traverse(v interface{}, f func(interface{}, Validator)) {
	f(v, a)
}

// here be dragons! change only if you know exactly what you're doing

// NOT thread safe
type RecursiveValidator struct {
	v Validator
}

func Recursion(f func(Validator) Validator) Validator {
	a := &RecursiveValidator{}
	a.Define(f(a))
	return a
}

func (r *RecursiveValidator) Validator() Validator {
	return r.v
}

func (r *RecursiveValidator) Validate(v interface{}, f []string) []Error {
	return r.v.Validate(v, f)
}

func (r *RecursiveValidator) Define(v Validator) {
	r.v = v
}

func (r *RecursiveValidator) Traverse(v interface{}, f func(interface{}, Validator)) {
	r.v.Traverse(v, f)
}

func uniqueErrors(es []Error) []Error {
	uq := make([]Error, 0, len(es))
outer:
	for _, e := range es {
		for _, q := range uq {
			if e.Equals(q) {
				continue outer
			}
		}
		uq = append(uq, e)
	}
	return uq
}

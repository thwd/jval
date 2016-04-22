package jval

import (
	"github.com/thwd/jsem"
	"math"
	"regexp"
	"strconv"
)

type Error struct {
	Label   string      `json:"label"`
	Field   string      `json:"field"`
	Context interface{} `json:"context"`
}

// Error implements native "error" interface
func (e Error) Error() string {
	return e.Label
}

var NoErrors = []Error{}

type Validator interface {
	Validate(value *jsem.Value, field string) []Error
}

type Lambda func(v *jsem.Value, f string) []Error

func (l Lambda) Validate(v *jsem.Value, f string) []Error {
	return l(v, f)
}

type AnythingValidator struct{}

func Anything() Validator {
	return AnythingValidator{}
}

func (a AnythingValidator) Validate(v *jsem.Value, f string) []Error {
	return NoErrors
}

type StringValidator struct{}

func String() Validator {
	return StringValidator{}
}

func (a StringValidator) Validate(v *jsem.Value, f string) []Error {
	if v.IsString() {
		return NoErrors
	}
	return []Error{Error{"value_must_be_string", f, nil}}
}

type NumberValidator struct{}

func Number() Validator {
	return NumberValidator{}
}

func (a NumberValidator) Validate(v *jsem.Value, f string) []Error {
	if v.IsNumber() {
		return NoErrors
	}
	return []Error{Error{"value_must_be_number", f, nil}}
}

type BooleanValidator struct{}

func Boolean() Validator {
	return BooleanValidator{}
}

func (a BooleanValidator) Validate(v *jsem.Value, f string) []Error {
	if v.IsBoolean() {
		return NoErrors
	}
	return []Error{Error{"value_must_be_boolean", f, nil}}
}

type NullValidator struct{}

func Null() Validator {
	return NullValidator{}
}

func (a NullValidator) Validate(v *jsem.Value, f string) []Error {
	if v.IsNull() {
		return NoErrors
	}
	return []Error{Error{"value_must_be_null", f, nil}}
}

type NotNullValidator struct{}

func NotNull() Validator {
	return NotNullValidator{}
}

func (a NotNullValidator) Validate(v *jsem.Value, f string) []Error {
	if v.IsNull() {
		return []Error{Error{"value_must_not_be_null", f, nil}}
	}
	return NoErrors
}

type AndValidator []Validator

func And(vs ...Validator) Validator {
	return AndValidator(vs)
}

func (a AndValidator) Validate(v *jsem.Value, f string) []Error {
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
	return []Error{Error{"and", "", ue}}
}
func (a AndValidator) Validators() []Validator {
	return []Validator(a)
}

type TupleValidator []Validator

func Tuple(vs ...Validator) Validator {
	return TupleValidator(vs)
}

func (a TupleValidator) Validate(v *jsem.Value, f string) []Error {
	return And(Array(Anything()), Length(len(a)), Lambda(func(v *jsem.Value, f string) []Error {
		for i, b := range a {
			u, _ := v.ArrayIndex(i)
			es := b.Validate(u, joinPaths(f, strconv.Itoa(i)))
			if len(es) != 0 {
				return es
			}
		}
		return NoErrors
	})).Validate(v, f)
}
func (a TupleValidator) Validators() []Validator {
	return []Validator(a)
}

type OrValidator []Validator

func Or(vs ...Validator) Validator {
	return OrValidator(vs)
}

func (b OrValidator) Validate(v *jsem.Value, f string) []Error {
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
	return []Error{Error{"or", "", ue}}
}

func (a OrValidator) Validators() []Validator {
	return []Validator(a)
}

type ObjectValidator map[string]Validator

func Object(d map[string]Validator) Validator {
	return ObjectValidator(d)
}

func (d ObjectValidator) Validate(v *jsem.Value, f string) []Error {
	if !v.IsObject() {
		return []Error{Error{"value_must_be_object", f, nil}}
	}
	ae := make([]Error, 0, len(d))
	v.ObjectForEach(func(k string, u *jsem.Value) {
		if _, ok := d[k]; !ok {
			ae = append(ae, Error{"unexpected_object_key", f, k})
		}
	})
	for k, a := range d {
		u, e := v.ObjectKey(k)
		if e != nil {
			ae = append(ae, Error{"missing_object_key", f, k})
			continue
		}
		ae = append(ae, a.Validate(u, joinPaths(f, k))...)
	}
	return ae
}

func (a ObjectValidator) Structure() map[string]Validator {
	return (map[string]Validator)(a)
}

type MapValidator struct {
	e Validator
}

func Map(e Validator) Validator {
	return MapValidator{e}
}

func (a MapValidator) Validate(v *jsem.Value, f string) []Error {
	if !v.IsObject() {
		return []Error{Error{"value_must_be_object", f, nil}}
	}
	ae := make([]Error, 0, 8)
	v.ObjectForEach(func(k string, u *jsem.Value) {
		ae = append(ae, a.e.Validate(u, joinPaths(f, k))...)
	})
	return ae
}
func (a MapValidator) Validator() Validator {
	return a.e
}

type ArrayValidator struct {
	e Validator
}

func Array(e Validator) Validator {
	return ArrayValidator{e}
}

func (a ArrayValidator) Validate(v *jsem.Value, f string) []Error {
	if !v.IsArray() {
		return []Error{Error{"value_must_be_array", f, nil}}
	}
	ae := make([]Error, 0, 8)
	v.ArrayForEach(func(i int, u *jsem.Value) {
		ae = append(ae, a.e.Validate(u, joinPaths(f, strconv.Itoa(i)))...)
	})
	return ae
}
func (a ArrayValidator) Validator() Validator {
	return a.e
}

type RegexValidator struct {
	l    string
	i, m bool
	x    string
	r    *regexp.Regexp
}

// see https://golang.org/pkg/regexp/syntax/
func Regex(x, l string, i, m bool) Validator {
	if i {
		x = `(?i)` + x
	}
	if m {
		x = `(?m)` + x
	}
	return RegexValidator{
		l, i, m, x, regexp.MustCompile(x),
	}
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
	return a.r
}

func (a RegexValidator) Validate(v *jsem.Value, f string) []Error {
	return And(String(), Lambda(func(v *jsem.Value, f string) []Error {
		s, _ := v.String()
		if a.r.MatchString(s) {
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

type RegexStringValidator struct{}

func RegexString() Validator {
	return RegexStringValidator{}
}

func (a RegexStringValidator) Validate(v *jsem.Value, f string) []Error {
	return And(String(), Lambda(func(v *jsem.Value, f string) []Error {
		s, _ := v.String()
		_, e := regexp.Compile(s)
		if e != nil {
			return []Error{Error{"value_must_be_regex_string", f, nil}}
		}
		return NoErrors
	})).Validate(v, f)
}

type OptionalValidator struct {
	e Validator
}

func Optional(e Validator) Validator {
	return OptionalValidator{e}
}

func (a OptionalValidator) Validate(v *jsem.Value, f string) []Error {
	if v == nil {
		return NoErrors
	}
	return a.e.Validate(v, f)
}
func (a OptionalValidator) Validator() Validator {
	return a.e
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

func (a LengthBetweenValidator) Validate(v *jsem.Value, f string) []Error {
	return And(Or(String(), Array(Anything())), Lambda(func(v *jsem.Value, f string) []Error {
		l := -1
		if v.IsArray() {
			l, _ = v.ArrayLength()
		} else {
			l, _ = v.StringLength()
		}
		if l < a.x || l > a.y {
			return []Error{
				Error{"value_must_have_length_between", f, map[string]int{"min": a.x, "max": a.y}},
			}
		}
		return NoErrors
	})).Validate(v, f)
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

func (a NumberBetweenValidator) Validate(v *jsem.Value, f string) []Error {
	return And(Number(), Lambda(func(v *jsem.Value, f string) []Error {
		l, _ := v.Float64()
		if l < a.x || l > a.y {
			return []Error{
				Error{"value_must_have_value_between", f, map[string]float64{"min": a.x, "max": a.y}},
			}
		}
		return NoErrors
	})).Validate(v, f)
}

type WholeNumberValidator struct{}

func WholeNumber() Validator {
	return WholeNumberValidator{}
}

func (a WholeNumberValidator) Validate(v *jsem.Value, f string) []Error {
	return And(Number(), Lambda(func(v *jsem.Value, f string) []Error {
		n, _ := v.Float64()
		_, r := math.Modf(n)
		if r != 0 {
			return []Error{Error{"value_must_be_whole_number", f, nil}}
		}
		return NoErrors
	})).Validate(v, f)
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

func (a WholeNumberBetweenValidator) Validate(v *jsem.Value, f string) []Error {
	return And(WholeNumber(), Lambda(func(v *jsem.Value, f string) []Error {
		l, _ := v.Int()
		if l < a.x || l > a.y {
			return []Error{
				Error{"value_must_have_value_between", f, map[string]int{"min": a.x, "max": a.y}},
			}
		}
		return NoErrors
	})).Validate(v, f)
}

type ExactlyValidator struct {
	j *jsem.Value
}

func Exactly(j *jsem.Value) Validator {
	return ExactlyValidator{j}
}

func (a ExactlyValidator) Value() *jsem.Value {
	return a.j
}

func (a ExactlyValidator) Validate(v *jsem.Value, f string) []Error {
	if !v.Equals(a.j) {
		return []Error{Error{"value_not_matched_exactly", f, nil}}
	}
	return NoErrors
}

// type CaseValidator []Validator

// func Case(cs ...Validator) Validator {
// 	return CaseValidator(cs)
// }

// func (a CaseValidator) Validators() []Validator {
// 	return []Validator(a)
// }

// func (a CaseValidator) Validate(v *jsem.Value, f string) []Error {
// 	return And(Tuple(WholeNumberBetween(0, len(a)-1), Anything()), Lambda(func(v *jsem.Value, f string) []Error {
// 		cv, _ := v.ArrayIndex(0)
// 		vv, _ := v.ArrayIndex(1)
// 		ix, _ := cv.Int()
// 		return a[ix].Validate(vv, joinPaths(f, "1"))
// 	})).Validate(v, f)
// }

func joinPaths(a, b string) string {
	if a == "" {
		return b
	}
	return a + "." + b
}

// here be dragons! use only if you know exactly what you're doing

type RecursiveValidator struct {
	v Validator
	l string
}

func (r *RecursiveValidator) Validator() Validator {
	return r.v
}

func (r *RecursiveValidator) Validate(v *jsem.Value, f string) []Error {
	return r.v.Validate(v, f)
}

func (r *RecursiveValidator) Define(v Validator) {
	r.v = v
}

func uniqueErrors(es []Error) []Error {
	uq := make([]Error, 0, len(es))
outer:
	for _, e := range es {
		for _, q := range uq {
			if e == q {
				continue outer
			}
		}
		uq = append(uq, e)
	}
	return uq
}

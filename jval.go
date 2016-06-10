package jval

import (
	"github.com/thwd/jsem"
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
	Validate(value *jsem.Value, field []string) []Error
	Traverse(*jsem.Value, func(*jsem.Value, Validator))
}

type Lambda func(v *jsem.Value, f []string) []Error

func (l Lambda) Validate(v *jsem.Value, f []string) []Error {
	return l(v, f)
}

func (l Lambda) Traverse(v *jsem.Value, f func(*jsem.Value, Validator)) {
	f(v, l)
}

type AnythingValidator struct{}

func Anything() Validator {
	return AnythingValidator{}
}

func (a AnythingValidator) Validate(v *jsem.Value, f []string) []Error {
	return NoErrors
}

func (a AnythingValidator) Traverse(v *jsem.Value, f func(*jsem.Value, Validator)) {
	f(v, a)
}

type StringValidator struct{}

func String() Validator {
	return StringValidator{}
}

func (a StringValidator) Validate(v *jsem.Value, f []string) []Error {
	if v.IsString() {
		return NoErrors
	}
	return []Error{Error{"value_must_be_string", f, nil}}
}

func (a StringValidator) Traverse(v *jsem.Value, f func(*jsem.Value, Validator)) {
	f(v, a)
}

type NumberValidator struct{}

func Number() Validator {
	return NumberValidator{}
}

func (a NumberValidator) Validate(v *jsem.Value, f []string) []Error {
	if v.IsNumber() {
		return NoErrors
	}
	return []Error{Error{"value_must_be_number", f, nil}}
}

func (a NumberValidator) Traverse(v *jsem.Value, f func(*jsem.Value, Validator)) {
	f(v, a)
}

type BooleanValidator struct{}

func Boolean() Validator {
	return BooleanValidator{}
}

func (a BooleanValidator) Validate(v *jsem.Value, f []string) []Error {
	if v.IsBoolean() {
		return NoErrors
	}
	return []Error{Error{"value_must_be_boolean", f, nil}}
}

func (a BooleanValidator) Traverse(v *jsem.Value, f func(*jsem.Value, Validator)) {
	f(v, a)
}

type NullValidator struct{}

func Null() Validator {
	return NullValidator{}
}

func (a NullValidator) Validate(v *jsem.Value, f []string) []Error {
	if v.IsNull() {
		return NoErrors
	}
	return []Error{Error{"value_must_be_null", f, nil}}
}

func (a NullValidator) Traverse(v *jsem.Value, f func(*jsem.Value, Validator)) {
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

func (a AndValidator) Validate(v *jsem.Value, f []string) []Error {
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

func (a AndValidator) Traverse(v *jsem.Value, f func(*jsem.Value, Validator)) {
	for _, b := range a {
		b.Traverse(v, f)
	}
}

// type TupleValidator []Validator

// func Tuple(vs ...Validator) Validator {
// 	return TupleValidator(vs)
// }

// func (a TupleValidator) Validate(v *jsem.Value, f []string) []Error {
// 	return And(Array(Anything()), Length(len(a)), Lambda(func(v *jsem.Value, f []string) []Error {
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

func (b OrValidator) Validate(v *jsem.Value, f []string) []Error {
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

func (a OrValidator) Traverse(v *jsem.Value, f func(*jsem.Value, Validator)) {
	for _, b := range a {
		b.Traverse(v, f)
	}
}

type CaseValidator map[string]Validator

func Case(d map[string]Validator) Validator {
	return CaseValidator(d)
}

func (d CaseValidator) Validate(v *jsem.Value, f []string) []Error {
	if !v.IsObject() {
		return []Error{Error{"value_must_be_object", f, nil}}
	}
	ks, _ := v.ObjectKeys()
	if len(ks) != 1 {
		return []Error{Error{"object_must_have_exactly_one_key", f, len(ks)}}
	}
	vd, k := d[ks[0]]
	if !k {
		return []Error{Error{"case_not_defined", f, ks[0]}}
	}
	tv, _ := v.ObjectKey(ks[0])
	return vd.Validate(tv, append(f, ks[0]))
}

func (a CaseValidator) Structure() map[string]Validator {
	return (map[string]Validator)(a)
}

func (a CaseValidator) Traverse(v *jsem.Value, f func(*jsem.Value, Validator)) {
	ks, _ := v.ObjectKeys()
	w, _ := v.ObjectKey(ks[0])
	a[ks[0]].Traverse(w, f)
}

type ObjectValidator map[string]Validator

func Object(d map[string]Validator) Validator {
	return ObjectValidator(d)
}

func (d ObjectValidator) Validate(v *jsem.Value, f []string) []Error {
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
		ae = append(ae, a.Validate(u, append(f, k))...)
	}
	return ae
}

func (a ObjectValidator) Structure() map[string]Validator {
	return (map[string]Validator)(a)
}

func (a ObjectValidator) Traverse(v *jsem.Value, f func(*jsem.Value, Validator)) {
	v.ObjectForEach(func(k string, w *jsem.Value) {
		a[k].Traverse(w, f)
	})
}

type MapValidator struct {
	e Validator
}

func Map(e Validator) Validator {
	return MapValidator{e}
}

func (a MapValidator) Validate(v *jsem.Value, f []string) []Error {
	if !v.IsObject() {
		return []Error{Error{"value_must_be_object", f, nil}}
	}
	ae := make([]Error, 0, 8)
	v.ObjectForEach(func(k string, u *jsem.Value) {
		ae = append(ae, a.e.Validate(u, append(f, k))...)
	})
	return ae
}
func (a MapValidator) Validator() Validator {
	return a.e
}

func (a MapValidator) Traverse(v *jsem.Value, f func(*jsem.Value, Validator)) {
	v.ForEach(func(v *jsem.Value) {
		a.e.Traverse(v, f)
	})
}

type ArrayValidator struct {
	e Validator
}

func Array(e Validator) Validator {
	return ArrayValidator{e}
}

func (a ArrayValidator) Validate(v *jsem.Value, f []string) []Error {
	if !v.IsArray() {
		return []Error{Error{"value_must_be_array", f, nil}}
	}
	ae := make([]Error, 0, 8)
	v.ArrayForEach(func(i int, u *jsem.Value) {
		ae = append(ae, a.e.Validate(u, append(f, strconv.Itoa(i)))...)
	})
	return ae
}
func (a ArrayValidator) Validator() Validator {
	return a.e
}

func (a ArrayValidator) Traverse(v *jsem.Value, f func(*jsem.Value, Validator)) {
	v.ForEach(func(v *jsem.Value) {
		a.e.Traverse(v, f)
	})
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

func (a RegexValidator) Validate(v *jsem.Value, f []string) []Error {
	return And(String(), Lambda(func(v *jsem.Value, f []string) []Error {
		s, _ := v.String()
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

func (a RegexValidator) Traverse(v *jsem.Value, f func(*jsem.Value, Validator)) {
	f(v, a)
}

// type RegexStringValidator struct{}

// func RegexString() Validator {
// 	return RegexStringValidator{}
// }

// func (a RegexStringValidator) Validate(v *jsem.Value, f []string) []Error {
// 	return And(String(), Lambda(func(v *jsem.Value, f []string) []Error {
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

func (a OptionalValidator) Validate(v *jsem.Value, f []string) []Error {
	if v == nil {
		return NoErrors
	}
	return a.e.Validate(v, f)
}
func (a OptionalValidator) Validator() Validator {
	return a.e
}

func (a OptionalValidator) Traverse(v *jsem.Value, f func(*jsem.Value, Validator)) {
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

func (a LengthBetweenValidator) Validate(v *jsem.Value, f []string) []Error {
	return And(Or(String(), Array(Anything())), Lambda(func(v *jsem.Value, f []string) []Error {
		l := -1
		if v.IsArray() {
			l, _ = v.ArrayLength()
		} else {
			l, _ = v.StringLength()
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

func (a LengthBetweenValidator) Traverse(v *jsem.Value, f func(*jsem.Value, Validator)) {
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

func (a NumberBetweenValidator) Validate(v *jsem.Value, f []string) []Error {
	return And(Number(), Lambda(func(v *jsem.Value, f []string) []Error {
		l, _ := v.Float64()
		if l < a.x || l > a.y {
			return []Error{
				Error{"value_must_have_value_between", f, map[string]float64{"min": a.x, "max": a.y}},
			}
		}
		return NoErrors
	})).Validate(v, f)
}

func (a NumberBetweenValidator) Traverse(v *jsem.Value, f func(*jsem.Value, Validator)) {
	f(v, a)
}

type WholeNumberValidator struct{}

func WholeNumber() Validator {
	return WholeNumberValidator{}
}

func (a WholeNumberValidator) Validate(v *jsem.Value, f []string) []Error {
	return And(Number(), Lambda(func(v *jsem.Value, f []string) []Error {
		n, _ := v.Float64()
		_, r := math.Modf(n)
		if r != 0 {
			return []Error{Error{"value_must_be_whole_number", f, nil}}
		}
		return NoErrors
	})).Validate(v, f)
}

func (a WholeNumberValidator) Traverse(v *jsem.Value, f func(*jsem.Value, Validator)) {
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

func (a WholeNumberBetweenValidator) Validate(v *jsem.Value, f []string) []Error {
	return And(WholeNumber(), Lambda(func(v *jsem.Value, f []string) []Error {
		l, _ := v.Int()
		if l < a.x || l > a.y {
			return []Error{
				Error{"value_must_have_value_between", f, map[string]int{"min": a.x, "max": a.y}},
			}
		}
		return NoErrors
	})).Validate(v, f)
}

func (a WholeNumberBetweenValidator) Traverse(v *jsem.Value, f func(*jsem.Value, Validator)) {
	f(v, a)
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

func (a ExactlyValidator) Validate(v *jsem.Value, f []string) []Error {
	if !v.Equals(a.j) {
		return []Error{Error{"value_not_matched_exactly", f, nil}}
	}
	return NoErrors
}

func (a ExactlyValidator) Traverse(v *jsem.Value, f func(*jsem.Value, Validator)) {
	f(v, a)
}

// here be dragons! change only if you know exactly what you're doing

// NOT thread safe
type RecursiveValidator struct {
	v Validator
	l bool
}

func Recursion(f func(Validator) Validator) Validator {
	a := &RecursiveValidator{}
	a.Define(f(a))
	return a
}

func (r *RecursiveValidator) Validator() Validator {
	return r.v
}

func (r *RecursiveValidator) Validate(v *jsem.Value, f []string) []Error {
	return r.v.Validate(v, f)
}

func (r *RecursiveValidator) Define(v Validator) {
	r.v = v
}

func (r *RecursiveValidator) Traverse(v *jsem.Value, f func(*jsem.Value, Validator)) {
	if r.l {
		return
	}
	r.l = true
	r.v.Traverse(v, f)
	r.l = false
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

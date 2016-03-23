package jval

import (
	"encoding/json"
	"github.com/thwd/jsem"
	"math"
	"math/rand"
	"regexp"
	"strconv"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type Error struct {
	Label   string      `json:"label"`
	Field   string      `json:"field"`
	Context interface{} `json:"context"`
}

// Error implements native "error" interface
func (e Error) Error() string {
	return e.Label
}

var noErrors = []Error{}

type Validator interface {
	Validate(value *jsem.Value, field string) []Error
	json.Marshaler
}

type lambda func(v *jsem.Value, f string) []Error

func (l lambda) Validate(v *jsem.Value, f string) []Error {
	return l(v, f)
}

func (l lambda) MarshalJSON() ([]byte, error) {
	return []byte(``), nil // dummy, never exposed
}

type AnythingValidator struct{}

func Anything() Validator {
	return AnythingValidator{}
}

func (a AnythingValidator) Validate(v *jsem.Value, f string) []Error {
	return noErrors
}

func (a AnythingValidator) MarshalJSON() ([]byte, error) {
	return []byte(`["anything",[]]`), nil
}

type StringValidator struct{}

func String() Validator {
	return StringValidator{}
}

func (a StringValidator) Validate(v *jsem.Value, f string) []Error {
	if v.IsString() {
		return noErrors
	}
	return []Error{Error{"value_must_be_string", f, nil}}
}

func (a StringValidator) MarshalJSON() ([]byte, error) {
	return []byte(`["string",[]]`), nil
}

type NumberValidator struct{}

func Number() Validator {
	return NumberValidator{}
}

func (a NumberValidator) Validate(v *jsem.Value, f string) []Error {
	if v.IsNumber() {
		return noErrors
	}
	return []Error{Error{"value_must_be_number", f, nil}}
}

func (a NumberValidator) MarshalJSON() ([]byte, error) {
	return []byte(`["number",[]]`), nil
}

type BooleanValidator struct{}

func Boolean() Validator {
	return BooleanValidator{}
}

func (a BooleanValidator) Validate(v *jsem.Value, f string) []Error {
	if v.IsBoolean() {
		return noErrors
	}
	return []Error{Error{"value_must_be_boolean", f, nil}}
}

func (a BooleanValidator) MarshalJSON() ([]byte, error) {
	return []byte(`["boolean",[]]`), nil
}

type NullValidator struct{}

func Null() Validator {
	return NullValidator{}
}

func (a NullValidator) Validate(v *jsem.Value, f string) []Error {
	if v.IsNull() {
		return noErrors
	}
	return []Error{Error{"value_must_be_null", f, nil}}
}

func (a NullValidator) MarshalJSON() ([]byte, error) {
	return []byte(`["null",[]]`), nil
}

type NotNullValidator struct{}

func NotNull() Validator {
	return NotNullValidator{}
}

func (a NotNullValidator) Validate(v *jsem.Value, f string) []Error {
	if v.IsNull() {
		return []Error{Error{"value_must_not_be_null", f, nil}}
	}
	return noErrors
}

func (a NotNullValidator) MarshalJSON() ([]byte, error) {
	return []byte(`["notNull",[]]`), nil
}

type AndValidator []Validator

func And(vs ...Validator) Validator {
	return AndValidator(vs)
}

func (a AndValidator) Validate(v *jsem.Value, f string) []Error {
	for _, b := range a {
		es := b.Validate(v, f)
		if len(es) != 0 {
			return es
		}
	}
	return noErrors
}

func (a AndValidator) MarshalJSON() ([]byte, error) {
	return json.Marshal([]interface{}{"and", []Validator(a)})
}

type TupleValidator []Validator

func Tuple(vs ...Validator) Validator {
	return TupleValidator(vs)
}

func (a TupleValidator) Validate(v *jsem.Value, f string) []Error {
	return And(Array(Anything()), Length(len(a)), lambda(func(v *jsem.Value, f string) []Error {
		for i, b := range a {
			u, _ := v.ArrayIndex(i)
			es := b.Validate(u, f+"."+strconv.Itoa(i))
			if len(es) != 0 {
				return es
			}
		}
		return noErrors
	})).Validate(v, f)
}

func (a TupleValidator) MarshalJSON() ([]byte, error) {
	return json.Marshal([]interface{}{"tuple", []Validator(a)})
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
			return noErrors
		}
		ae = append(ae, es...)
	}
	return ae // TODO(thwd): merge errors into one?
}

func (a OrValidator) MarshalJSON() ([]byte, error) {
	return json.Marshal([]interface{}{"or", []Validator(a)})
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
			ae = append(ae, Error{"unexpected_object_key", f + "." + k, nil})
		}
	})
	for k, a := range d {
		u, _ := v.ObjectKey(k)
		ae = append(ae, a.Validate(u, f+"."+k)...)
	}
	return ae
}

func (a ObjectValidator) MarshalJSON() ([]byte, error) {
	return json.Marshal([]interface{}{"object", []map[string]Validator{map[string]Validator(a)}})
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
		ae = append(ae, a.e.Validate(u, f+"."+k)...)
	})
	return ae
}

func (a MapValidator) MarshalJSON() ([]byte, error) {
	return json.Marshal([]interface{}{"map", []Validator{a.e}})
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
		ae = append(ae, a.e.Validate(u, f+"."+strconv.Itoa(i))...)
	})
	return ae
}

func (a ArrayValidator) MarshalJSON() ([]byte, error) {
	return json.Marshal([]interface{}{"array", []Validator{a.e}})
}

type RegexValidator struct {
	r *regexp.Regexp
}

func Regex(r *regexp.Regexp) Validator {
	return RegexValidator{r}
}

func (a RegexValidator) Validate(v *jsem.Value, f string) []Error {
	return And(String(), lambda(func(v *jsem.Value, f string) []Error {
		s, _ := v.String()
		if a.r.Match([]byte(s)) {
			return noErrors
		}
		return []Error{
			Error{"value_must_match_regex", f, map[string]string{"regex": a.r.String()}},
		}
	})).Validate(v, f)
}

func (a RegexValidator) MarshalJSON() ([]byte, error) {
	return json.Marshal([]interface{}{"regex", []string{a.r.String()}})
}

type OptionalValidator struct {
	e Validator
}

func Optional(e Validator) Validator {
	return OptionalValidator{e}
}

func (a OptionalValidator) Validate(v *jsem.Value, f string) []Error {
	if v == nil {
		return noErrors
	}
	return a.e.Validate(v, f)
}

func (a OptionalValidator) MarshalJSON() ([]byte, error) {
	return json.Marshal([]interface{}{"optional", []Validator{a.e}})
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
	return And(Or(String(), Array(Anything())), lambda(func(v *jsem.Value, f string) []Error {
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
		return noErrors
	})).Validate(v, f)
}

func (a LengthBetweenValidator) MarshalJSON() ([]byte, error) {
	return json.Marshal([]interface{}{"lengthBetween", []int{a.x, a.y}})
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
	return And(Number(), lambda(func(v *jsem.Value, f string) []Error {
		l, _ := v.Float64()
		if l < a.x || l > a.y {
			return []Error{
				Error{"value_must_have_value_between", f, map[string]float64{"min": a.x, "max": a.y}},
			}
		}
		return noErrors
	})).Validate(v, f)
}

func (a NumberBetweenValidator) MarshalJSON() ([]byte, error) {
	return json.Marshal([]interface{}{"numberBetween", []float64{a.x, a.y}})
}

type WholeNumberValidator struct{}

func WholeNumber() Validator {
	return WholeNumberValidator{}
}

func (a WholeNumberValidator) Validate(v *jsem.Value, f string) []Error {
	return And(Number(), lambda(func(v *jsem.Value, f string) []Error {
		n, _ := v.Float64()
		_, r := math.Modf(n)
		if r != 0 {
			return []Error{Error{"value_must_be_whole_number", f, nil}}
		}
		return noErrors
	})).Validate(v, f)
}

func (a WholeNumberValidator) MarshalJSON() ([]byte, error) {
	return []byte(`["wholeNumber",[]]`), nil
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
	return And(WholeNumber(), lambda(func(v *jsem.Value, f string) []Error {
		l, _ := v.Int()
		if l < a.x || l > a.y {
			return []Error{
				Error{"value_must_have_value_between", f, map[string]int{"min": a.x, "max": a.y}},
			}
		}
		return noErrors
	})).Validate(v, f)
}

func (a WholeNumberBetweenValidator) MarshalJSON() ([]byte, error) {
	return json.Marshal([]interface{}{"wholeNumberBetween", []int{a.x, a.y}})
}

type ExactlyValidator struct {
	j *jsem.Value
}

func Exactly(j *jsem.Value) Validator {
	return ExactlyValidator{j}
}

func (a ExactlyValidator) Validate(v *jsem.Value, f string) []Error {
	if !v.Equals(a.j) {
		return []Error{Error{"value_not_matched_exactly", f, nil}}
	}
	return noErrors
}

func (a ExactlyValidator) MarshalJSON() ([]byte, error) {
	return json.Marshal([]interface{}{"exactly", []*jsem.Value{a.j}})
}

type CaseValidator []Validator

func Case(cs ...Validator) Validator {
	return CaseValidator(cs)
}

func (a CaseValidator) Validate(v *jsem.Value, f string) []Error {
	return And(Tuple(WholeNumberBetween(0, len(a)-1), Anything()), lambda(func(v *jsem.Value, f string) []Error {
		cv, _ := v.ArrayIndex(0)
		vv, _ := v.ArrayIndex(1)
		ix, _ := cv.Int()
		return a[ix].Validate(vv, f+".1")
	})).Validate(v, f)
}

func (a CaseValidator) MarshalJSON() ([]byte, error) {
	return json.Marshal([]interface{}{"case", []Validator(a)})
}

// here be dragons! use only if you know exactly what you're doing

type RecursiveValidator struct {
	v Validator
	l string
}

func (r *RecursiveValidator) Validate(v *jsem.Value, f string) []Error {
	return r.v.Validate(v, f)
}

func (r *RecursiveValidator) Define(v Validator) {
	r.v = v
}

func (a *RecursiveValidator) MarshalJSON() ([]byte, error) {
	if a.v == nil {
		return nil, nil
	}
	if a.l != "" {
		return json.Marshal([]interface{}{"recurse", []interface{}{a.l}})
	}
	a.l = strconv.Itoa(rand.Int())
	bs, e := json.Marshal([]interface{}{"recursion", []interface{}{a.l, a.v}})
	a.l = ""
	return bs, e
}

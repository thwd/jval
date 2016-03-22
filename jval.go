package jval

import (
	"bytes"
	"encoding/json"
	"github.com/thwd/jsem"
	"math/rand"
	"regexp"
	"strconv"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type Error struct {
	Label   string
	Field   string
	Context interface{}
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
	bf := bytes.NewBuffer(nil)
	bf.WriteString(`["and",[`)
	for i, l := 0, len(a); i < l; i++ {
		bs, _ := a[i].MarshalJSON()
		bf.Write(bs)
		if i < (l - 1) {
			bf.WriteString(`,`)
		}
	}
	bf.WriteString(`]]`)
	return bf.Bytes(), nil
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
	bf := bytes.NewBuffer(nil)
	bf.WriteString(`["tuple",[`)
	for i, l := 0, len(a); i < l; i++ {
		bs, _ := a[i].MarshalJSON()
		bf.Write(bs)
		if i < (l - 1) {
			bf.WriteString(`,`)
		}
	}
	bf.WriteString(`]]`)
	return bf.Bytes(), nil
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
	bf := bytes.NewBuffer(nil)
	bf.WriteString(`["or",[`)
	for i, l := 0, len(a); i < l; i++ {
		bs, _ := a[i].MarshalJSON()
		bf.Write(bs)
		if i < (l - 1) {
			bf.WriteString(`,`)
		}
	}
	bf.WriteString(`]]`)
	return bf.Bytes(), nil
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
	bf := bytes.NewBuffer(nil)
	bf.WriteString(`["object",[`)
	json.NewEncoder(bf).Encode(map[string]Validator(a)) // TODO(thwd): TEST THIS!
	bf.WriteString(`]]`)
	return bf.Bytes(), nil
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
	bf := bytes.NewBuffer(nil)
	bf.WriteString(`["map",[`)
	bs, _ := a.e.MarshalJSON()
	bf.Write(bs)
	bf.WriteString(`]]`)
	return bf.Bytes(), nil
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
	bf := bytes.NewBuffer(nil)
	bf.WriteString(`["array",[`)
	bs, _ := a.e.MarshalJSON()
	bf.Write(bs)
	bf.WriteString(`]]`)
	return bf.Bytes(), nil
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
	bf := bytes.NewBuffer(nil)
	bf.WriteString(`["regex",[`)
	json.NewEncoder(bf).Encode(a.r.String())
	bf.WriteString(`]]`)
	return bf.Bytes(), nil
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
	bf := bytes.NewBuffer(nil)
	bf.WriteString(`["optional",[`)
	bs, _ := a.e.MarshalJSON()
	bf.Write(bs)
	bf.WriteString(`]]`)
	return bf.Bytes(), nil
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
	bf := bytes.NewBuffer(nil)
	bf.WriteString(`["lengthBetween",[`)
	json.NewEncoder(bf).Encode(a.x)
	bf.WriteString(`,`)
	json.NewEncoder(bf).Encode(a.y)
	bf.WriteString(`]]`)
	return bf.Bytes(), nil
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
	bf := bytes.NewBuffer(nil)
	bf.WriteString(`["numberBetween",[`)
	json.NewEncoder(bf).Encode(a.x)
	bf.WriteString(`,`)
	json.NewEncoder(bf).Encode(a.y)
	bf.WriteString(`]]`)
	return bf.Bytes(), nil
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
	bf := bytes.NewBuffer(nil)
	bf.WriteString(`["exactly",[`)
	bs, _ := a.j.MarshalJSON()
	bf.Write(bs)
	bf.WriteString(`]]`)
	return bf.Bytes(), nil
}

// here be dragons! use only if you know exactly what you're doing

type RecursiveValidator struct {
	v Validator
	l int
}

func (r *RecursiveValidator) Validate(v *jsem.Value, f string) []Error {
	return r.v.Validate(v, f)
}

func (r *RecursiveValidator) Define(v Validator) {
	r.v = v
}

func (a *RecursiveValidator) MarshalJSON() ([]byte, error) {
	bf := bytes.NewBuffer(nil)
	if a.l == 0 {
		bf := bytes.NewBuffer(nil)
		a.l = rand.Int()
		bf.WriteString(`["recursion",[`)
		json.NewEncoder(bf).Encode(a.l)
		bf.WriteString(`,`)
		bs, _ := a.v.MarshalJSON()
		bf.Write(bs)
		a.l = 0
		bf.WriteString(`]]`)
		return bf.Bytes(), nil
	}
	bf.WriteString(`["recurse",[`)
	json.NewEncoder(bf).Encode(a.l)
	bf.WriteString(`]]`)
	return bf.Bytes(), nil
}

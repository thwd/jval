package jval

import (
	"github.com/thwd/jsem"
	"regexp"
	"strconv"
)

type Error interface {
	Error() string
	Field() string
	AdditionalInformation() interface{}
}

var noErrors = []Error{}

type commonError struct {
	e, f string
	a    interface{}
}

func (e commonError) Error() string {
	return e.e
}

func (e commonError) Field() string {
	return e.f
}

func (e commonError) AdditionalInformation() interface{} {
	return e.a
}

type Validator interface {
	Validate(value *jsem.Value, field string) []Error
}

type Lambda func(v *jsem.Value, f string) []Error

func (l Lambda) Validate(v *jsem.Value, f string) []Error {
	return l(v, f)
}

var String Validator = Lambda(func(v *jsem.Value, f string) []Error {
	if v.IsString() {
		return noErrors
	}
	return []Error{commonError{"value_must_be_string", f, nil}}
})

var Number Validator = Lambda(func(v *jsem.Value, f string) []Error {
	if v.IsNumber() {
		return noErrors
	}
	return []Error{commonError{"value_must_be_number", f, nil}}
})

var Boolean Validator = Lambda(func(v *jsem.Value, f string) []Error {
	if v.IsBoolean() {
		return noErrors
	}
	return []Error{commonError{"value_must_be_boolean", f, nil}}
})

func Regex(r *regexp.Regexp) Validator {
	return And(String, Lambda(func(v *jsem.Value, f string) []Error {
		s, _ := v.String()
		if r.Match([]byte(s)) {
			return noErrors
		}
		return []Error{
			commonError{"value_must_match_regex", f, map[string]string{"regex": r.String()}},
		}
	}))
}

func And(vs ...Validator) Validator {
	return Lambda(func(v *jsem.Value, f string) []Error {
		for _, a := range vs {
			es := a.Validate(v, f)
			if len(es) != 0 {
				return es
			}
		}
		return noErrors
	})
}

func Or(vs ...Validator) Validator {
	return Lambda(func(v *jsem.Value, f string) []Error {
		ae := make([]Error, 0, len(vs))
		for _, a := range vs {
			ae = append(ae, a.Validate(v, f)...)
		}
		return ae
	})
}

type ObjectOptions struct {
	AllowNull      bool
	AllowOtherKeys bool
}

type ObjectDefinition map[string]Validator

func Object(o ObjectOptions, d ObjectDefinition) Validator {
	return Lambda(func(v *jsem.Value, f string) []Error {
		if o.AllowNull && v.IsNull() {
			return noErrors
		}
		if !o.AllowNull && v.IsNull() {
			return []Error{commonError{"value_must_not_be_null", f, nil}}
		}
		if !v.IsObject() {
			return []Error{commonError{"value_must_be_object", f, nil}}
		}
		ae := make([]Error, 0, len(d))
		if !o.AllowOtherKeys {
			v.ObjectForEach(func(k string, u *jsem.Value) {
				if _, ok := d[k]; !ok {
					ae = append(ae, commonError{"unallowed_object_key", f + "." + k, nil})
				}
			})
		}
		for k, a := range d {
			u, _ := v.ObjectKey(k)
			ae = append(ae, a.Validate(u, f+"."+k)...)
		}
		return ae
	})
}

type ArrayOptions struct {
	AllowNull bool
}

func Array(o ArrayOptions, e Validator) Validator {
	return Lambda(func(v *jsem.Value, f string) []Error {
		if o.AllowNull && v.IsNull() {
			return noErrors
		}
		if !o.AllowNull && v.IsNull() {
			return []Error{commonError{"value_must_not_be_null", f, nil}}
		}
		if !v.IsArray() {
			return []Error{commonError{"value_must_be_array", f, nil}}
		}
		ae := make([]Error, 0, 8)
		v.ArrayForEach(func(i int, u *jsem.Value) {
			ae = append(ae, e.Validate(u, f+"."+strconv.Itoa(i))...)
		})
		return ae
	})
}

func Optional(a Validator) Validator {
	return Lambda(func(v *jsem.Value, f string) []Error {
		if v == nil {
			return noErrors
		}
		return a.Validate(v, f)
	})
}

var Anything Validator = Lambda(func(v *jsem.Value, f string) []Error {
	return noErrors
})

func LengthBetween(x, y int) Validator {
	if y < x {
		panic("LengthBetween: y < x")
	}
	return And(Or(String, Array(ArrayOptions{AllowNull: false}, Anything)), Lambda(func(v *jsem.Value, f string) []Error {
		l := -1
		if v.IsArray() {
			l, _ = v.ArrayLength()
		} else {
			l, _ = v.StringLength()
		}
		if l < x || l > y {
			return []Error{
				commonError{"value_must_have_length_between", f, map[string]int{"min": x, "max": y}},
			}
		}
		return noErrors
	}))
}

func NumberBetween(x, y float64) Validator {
	if y < x {
		panic("NumberBetween: y < x")
	}
	return And(Number, Lambda(func(v *jsem.Value, f string) []Error {
		l, _ := v.Float64()
		if l < x || l > y {
			return []Error{
				commonError{"value_must_have_value_between", f, map[string]float64{"min": x, "max": y}},
			}
		}
		return noErrors
	}))
}

func Exactly(j *jsem.Value) Validator {
	return Lambda(func(v *jsem.Value, f string) []Error {
		if !v.Equals(j) {
			return []Error{commonError{"value_not_matched_exactly", f, nil}}
		}
		return noErrors
	})
}

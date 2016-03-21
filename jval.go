package jval

import (
	"github.com/thwd/jsem"
	"regexp"
	"strconv"
)

type Error interface {
	Error() string
	Field() string
	Context() interface{}
}

var noErrors = []Error{}

type err struct {
	e, f string
	c    interface{}
}

func (e err) Error() string {
	return e.e
}

func (e err) Field() string {
	return e.f
}

func (e err) Context() interface{} {
	return e.c
}

type Validator interface {
	Validate(value *jsem.Value, field string) []Error
}

type Lambda func(v *jsem.Value, f string) []Error

func (l Lambda) Validate(v *jsem.Value, f string) []Error {
	return l(v, f)
}

var Anything Validator = Lambda(func(v *jsem.Value, f string) []Error {
	return noErrors
})

var String Validator = Lambda(func(v *jsem.Value, f string) []Error {
	if v.IsString() {
		return noErrors
	}
	return []Error{err{"value_must_be_string", f, nil}}
})

var Number Validator = Lambda(func(v *jsem.Value, f string) []Error {
	if v.IsNumber() {
		return noErrors
	}
	return []Error{err{"value_must_be_number", f, nil}}
})

var Boolean Validator = Lambda(func(v *jsem.Value, f string) []Error {
	if v.IsBoolean() {
		return noErrors
	}
	return []Error{err{"value_must_be_boolean", f, nil}}
})

var Null Validator = Lambda(func(v *jsem.Value, f string) []Error {
	if v.IsNull() {
		return noErrors
	}
	return []Error{err{"value_must_be_null", f, nil}}
})

var NotNull Validator = Lambda(func(v *jsem.Value, f string) []Error {
	if v.IsNull() {
		return []Error{err{"value_must_not_be_null", f, nil}}
	}
	return noErrors
})

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
			es := a.Validate(v, f)
			if len(es) == 0 {
				return noErrors
			}
			ae = append(ae, es...)
		}
		return ae // TODO(thwd): merge errors into one?
	})
}

func Object(d map[string]Validator) Validator {
	return Lambda(func(v *jsem.Value, f string) []Error {
		if !v.IsObject() {
			return []Error{err{"value_must_be_object", f, nil}}
		}
		ae := make([]Error, 0, len(d))
		v.ObjectForEach(func(k string, u *jsem.Value) {
			if _, ok := d[k]; !ok {
				ae = append(ae, err{"unexpected_object_key", f + "." + k, nil})
			}
		})
		for k, a := range d {
			u, _ := v.ObjectKey(k)
			ae = append(ae, a.Validate(u, f+"."+k)...)
		}
		return ae
	})
}

func Array(e Validator) Validator {
	return Lambda(func(v *jsem.Value, f string) []Error {
		if !v.IsArray() {
			return []Error{err{"value_must_be_array", f, nil}}
		}
		ae := make([]Error, 0, 8)
		v.ArrayForEach(func(i int, u *jsem.Value) {
			ae = append(ae, e.Validate(u, f+"."+strconv.Itoa(i))...)
		})
		return ae
	})
}

func Regex(r *regexp.Regexp) Validator {
	return And(String, Lambda(func(v *jsem.Value, f string) []Error {
		s, _ := v.String()
		if r.Match([]byte(s)) {
			return noErrors
		}
		return []Error{
			err{"value_must_match_regex", f, map[string]string{"regex": r.String()}},
		}
	}))
}

func Optional(a Validator) Validator {
	return Lambda(func(v *jsem.Value, f string) []Error {
		if v == nil {
			return noErrors
		}
		return a.Validate(v, f)
	})
}

func LengthBetween(x, y int) Validator {
	if y < x {
		panic("LengthBetween: y < x")
	}
	return And(Or(String, Array(Anything)), Lambda(func(v *jsem.Value, f string) []Error {
		l := -1
		if v.IsArray() {
			l, _ = v.ArrayLength()
		} else {
			l, _ = v.StringLength()
		}
		if l < x || l > y {
			return []Error{
				err{"value_must_have_length_between", f, map[string]int{"min": x, "max": y}},
			}
		}
		return noErrors
	}))
}

func Length(x int) Validator {
	return LengthBetween(x, x)
}

func NumberBetween(x, y float64) Validator {
	if y < x {
		panic("NumberBetween: y < x")
	}
	return And(Number, Lambda(func(v *jsem.Value, f string) []Error {
		l, _ := v.Float64()
		if l < x || l > y {
			return []Error{
				err{"value_must_have_value_between", f, map[string]float64{"min": x, "max": y}},
			}
		}
		return noErrors
	}))
}

func Exactly(j *jsem.Value) Validator {
	return Lambda(func(v *jsem.Value, f string) []Error {
		if !v.Equals(j) {
			return []Error{err{"value_not_matched_exactly", f, nil}}
		}
		return noErrors
	})
}

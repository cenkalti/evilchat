package envconfig

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// A ParseError occurs when an environment variable cannot be converted to
// the type required by a struct field during assignment.
type ParseError struct {
	KeyName   string
	FieldName string
	TypeName  string
	Value     string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("config: assigning %[1]s to %[2]s: converting '%[3]s' to type %[4]s", e.KeyName, e.FieldName, e.Value, e.TypeName)
}

// EnvEmptyError occurs when Process function is called with mustExist=true and an environment variable is not set or empty.
type EnvEmptyError struct {
	KeyName string
}

func (e *EnvEmptyError) Error() string {
	return fmt.Sprintf("config: environment variable %s is not set", e.KeyName)
}

// ErrInvalidSpecification indicates that a specification is of the wrong type.
var ErrInvalidSpecification = errors.New("invalid specification must be a struct")

func Process(spec interface{}, mustExist bool) error {
	s := reflect.ValueOf(spec).Elem()
	if s.Kind() != reflect.Struct {
		return ErrInvalidSpecification
	}
	typeOfSpec := s.Type()
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		if f.CanSet() {
			var fieldName string
			alt := typeOfSpec.Field(i).Tag.Get("env")
			if alt != "" {
				fieldName = alt
			} else {
				fieldName = typeOfSpec.Field(i).Name
			}
			key := strings.ToUpper(fieldName)
			value := os.Getenv(key)
			if value == "" {
				if mustExist {
					return &EnvEmptyError{KeyName: key}
				} else {
					value = typeOfSpec.Field(i).Tag.Get("default")
				}
			}
			switch f.Kind() {
			case reflect.String:
				f.SetString(value)
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				intValue, err := strconv.ParseInt(value, 0, f.Type().Bits())
				if err != nil {
					return &ParseError{
						KeyName:   key,
						FieldName: fieldName,
						TypeName:  f.Type().String(),
						Value:     value,
					}
				}
				f.SetInt(intValue)
			case reflect.Bool:
				boolValue, err := strconv.ParseBool(value)
				if err != nil {
					return &ParseError{
						KeyName:   key,
						FieldName: fieldName,
						TypeName:  f.Type().String(),
						Value:     value,
					}
				}
				f.SetBool(boolValue)
			case reflect.Float32:
				floatValue, err := strconv.ParseFloat(value, f.Type().Bits())
				if err != nil {
					return &ParseError{
						KeyName:   key,
						FieldName: fieldName,
						TypeName:  f.Type().String(),
						Value:     value,
					}
				}
				f.SetFloat(floatValue)
			}
		}
	}
	return nil
}

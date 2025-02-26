package utfunc

import (
	"fmt"
	"reflect"
	"strings"

	"code.finan.one/finan-one-be/fo-utils/utils/customtype"
	"github.com/go-playground/validator/v10"
)

// CustomValidator is a custom validator that uses the "valid" tag
type CustomValidator struct {
	validate *validator.Validate
}

// ValidateStruct validates a struct based on the "valid" tag
func (cv *CustomValidator) ValidateStruct(s interface{}) error {
	return cv.validate.Struct(s)
}

func validateCustomTypeTime(fl validator.FieldLevel) bool {
	value := fl.Field().Interface().(customtype.Time)
	if value.IsZero() {
		return false
	}
	return true
}

// NewCustomValidator creates a new CustomValidator instance
func NewCustomValidator() *CustomValidator {
	v := validator.New()
	v.SetTagName("valid") // Set the tag name to "valid"
	v.RegisterValidation("time", validateCustomTypeTime)
	return &CustomValidator{v}
}

/*
CheckValidateStruct validates the fields of the given struct using the validator package.

Parameters:

obj interface{}: A pointer to the struct to be validated.

Returns:

error: If validation succeeds, nil is returned. Otherwise, an error is returned containing a comma-separated list of field names and their validation errors.

Example:

	type Person struct {
	    Name string `json:"name" validate:"required"`
	    Age  int    `json:"age" validate:"gte=0,lte=120"`
	}

	p := &Person{
	    Name: "",
	    Age:  250,
	}

	err := CheckValidateStruct(p)
	if err != nil {
	    fmt.Println(err) // Name: required, Age: lte
	}
*/
func CheckValidateStruct(obj interface{}) error {
	v := NewCustomValidator()
	return validateStruct(obj, v)
}

// validateStruct is a helper function that performs the actual struct validation using the validator package.
func validateStruct(obj interface{}, v *CustomValidator) error {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occurred during validation: %v", r)
		}
	}()

	err = v.ValidateStruct(obj)
	if err != nil {
		var errorMessages []string
		for _, e := range err.(validator.ValidationErrors) {
			fieldName := getFieldJSONTag(obj, e.Field())
			if fieldName == "" {
				fieldName = e.Field()
			}
			println(e.ActualTag())
			if e.Tag() != "required" {
				errorMessages = append(errorMessages, fmt.Sprintf("%s must %s %s", fieldName, e.Tag(), e.Param()))
			} else {
				errorMessages = append(errorMessages, fmt.Sprintf("%s is %s", fieldName, e.Tag()))
			}
		}
		return fmt.Errorf(strings.Join(errorMessages, ", "))
	}
	return nil
}

// getFieldJSONTag is a helper function that retrieves the JSON tag of a field in a struct.
func getFieldJSONTag(obj interface{}, fieldName string) string {
	t := reflect.TypeOf(obj)
	if t.Kind() == reflect.Struct {
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if field.Name == fieldName {
				return field.Tag.Get("json")
			}
		}
	}
	return fieldName
}

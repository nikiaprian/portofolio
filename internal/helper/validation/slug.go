package validation

import (
	"reflect"
	"regexp"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func validateSlug(fl validator.FieldLevel) bool {
	var s string
	switch fl.Field().Kind() {
	case reflect.String:
		s = fl.Field().String()
	case reflect.Ptr:
		if fl.Field().IsNil() {
			return true
		}
		el := fl.Field().Elem()
		if el.Kind() != reflect.String {
			return false
		}
		s = el.String()
	default:
		return false
	}
	if s == "" {
		return true
	}
	return slugPattern.MatchString(s)
}

// RegisterSlugValidator registers the "slug" tag used on category, tag, and post DTOs.
func RegisterSlugValidator() {
	if eng, ok := binding.Validator.Engine().(*validator.Validate); ok {
		_ = eng.RegisterValidation("slug", validateSlug)
	}
}

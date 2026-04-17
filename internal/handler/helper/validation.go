package helper

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type ValidationErrors map[string]string

var (
	validate     *validator.Validate
	validateOnce sync.Once
)

func DecodeAndValidateJSON(c *gin.Context, target any) bool {
	decoder := json.NewDecoder(c.Request.Body)
	if err := decoder.Decode(target); err != nil {
		RespondInvalidRequest(c)
		return false
	}

	var extra struct{}
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		RespondInvalidRequest(c)
		return false
	}

	validationErrors := ValidateStruct(target)
	if len(validationErrors) > 0 {
		RespondValidationErrors(c, validationErrors)
		return false
	}

	return true
}

func ValidateStruct(target any) ValidationErrors {
	err := validatorInstance().Struct(target)
	if err == nil {
		return nil
	}

	var fieldErrors validator.ValidationErrors
	if !errors.As(err, &fieldErrors) {
		return ValidationErrors{
			"request": err.Error(),
		}
	}

	response := make(ValidationErrors, len(fieldErrors))
	for _, fieldError := range fieldErrors {
		response[fieldError.Field()] = fieldError.Error()
	}

	return response
}

func RespondInvalidRequest(c *gin.Context) {
	c.JSON(http.StatusBadRequest, gin.H{
		"error": "invalid request",
	})
}

func RespondValidationErrors(c *gin.Context, validationErrors ValidationErrors) {
	c.JSON(http.StatusUnprocessableEntity, gin.H{
		"errors": validationErrors,
	})
}

func validatorInstance() *validator.Validate {
	validateOnce.Do(func() {
		validate = validator.New()
		validate.SetTagName("validate")
		validate.RegisterTagNameFunc(func(field reflect.StructField) string {
			name := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			if name != "" {
				return name
			}

			return field.Name
		})
	})

	return validate
}

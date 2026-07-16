// contact_editor.go - проверка ручных изменений рабочего справочника.
package services

import (
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"task1/models"
	"task1/utils"
)

const (
	maxContactNameLength  = 500
	maxContactEmailLength = 320
)

var (
	ErrContactPhoneInvalid    = errors.New("Введите корректный российский номер телефона")
	ErrContactEmailInvalid    = errors.New("Введите корректный email или оставьте поле пустым")
	ErrContactDiscountInvalid = errors.New("Скидка должна быть числом от 0 до 100")
	ErrContactNameTooLong     = errors.New("ФИО не должно превышать 500 символов")
	ErrContactValueInvalid    = errors.New("Поля контакта не должны содержать управляющие символы")
	ErrContactVersionInvalid  = errors.New("Версия контакта отсутствует или имеет неверный формат")
)

// ValidateContactUpdate нормализует редактируемые поля до обращения к PostgreSQL.
func ValidateContactUpdate(request models.ContactUpdateRequest) (models.Contact, time.Time, error) {
	phone, ok := utils.NormalizePhone(request.Phone)
	if !ok {
		return models.Contact{}, time.Time{}, ErrContactPhoneInvalid
	}

	name := utils.CleanCell(request.Name)
	if utf8.RuneCountInString(name) > maxContactNameLength {
		return models.Contact{}, time.Time{}, ErrContactNameTooLong
	}

	email := strings.TrimSpace(request.Email)
	if email != "" {
		email, ok = utils.NormalizeEmail(email)
		if !ok || utf8.RuneCountInString(email) > maxContactEmailLength {
			return models.Contact{}, time.Time{}, ErrContactEmailInvalid
		}
	}

	discount := strings.TrimSpace(request.Discount)
	if discount != "" {
		discount, ok = utils.NormalizePercent(discount)
		if !ok {
			return models.Contact{}, time.Time{}, ErrContactDiscountInvalid
		}
	}

	if containsControl(name) || containsControl(email) || containsControl(discount) {
		return models.Contact{}, time.Time{}, ErrContactValueInvalid
	}

	version, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(request.Version))
	if err != nil || version.IsZero() {
		return models.Contact{}, time.Time{}, ErrContactVersionInvalid
	}

	return models.Contact{
		Phone:    phone,
		Email:    email,
		Name:     name,
		Discount: discount,
	}, version, nil
}

func containsControl(value string) bool {
	return strings.ContainsFunc(value, unicode.IsControl)
}

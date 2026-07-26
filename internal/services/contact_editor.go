package services

import (
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"task1/internal/contactdata"
	"task1/internal/models"
	"task1/internal/tabular"
)

const (
	maxContactNameLength  = 500
	maxContactEmailLength = 320
)

var (
	// ErrContactPhoneInvalid сообщает о некорректном российском номере.
	ErrContactPhoneInvalid = errors.New("Введите корректный российский номер телефона")
	// ErrContactEmailInvalid сообщает о некорректном непустом email.
	ErrContactEmailInvalid = errors.New("Введите корректный email или оставьте поле пустым")
	// ErrContactDiscountInvalid сообщает о скидке вне допустимого диапазона.
	ErrContactDiscountInvalid = errors.New("Скидка должна быть числом от 0 до 100")
	// ErrContactNameTooLong сообщает о превышении ограничения длины имени.
	ErrContactNameTooLong = errors.New("ФИО не должно превышать 500 символов")
	// ErrContactValueInvalid сообщает об управляющих символах в полях.
	ErrContactValueInvalid = errors.New("Поля контакта не должны содержать управляющие символы")
	// ErrContactVersionInvalid сообщает об отсутствующей или некорректной версии.
	ErrContactVersionInvalid = errors.New("Версия контакта отсутствует или имеет неверный формат")
)

// ValidateContactUpdate нормализует редактируемые поля до обращения к PostgreSQL.
func ValidateContactUpdate(request models.ContactUpdateRequest) (models.Contact, time.Time, error) {
	phone, ok := contactdata.NormalizePhone(request.Phone)
	if !ok {
		return models.Contact{}, time.Time{}, ErrContactPhoneInvalid
	}

	name := tabular.CleanCell(request.Name)
	if utf8.RuneCountInString(name) > maxContactNameLength {
		return models.Contact{}, time.Time{}, ErrContactNameTooLong
	}

	email := strings.TrimSpace(request.Email)
	if email != "" {
		email, ok = contactdata.NormalizeEmail(email)
		if !ok || utf8.RuneCountInString(email) > maxContactEmailLength {
			return models.Contact{}, time.Time{}, ErrContactEmailInvalid
		}
	}

	discount := strings.TrimSpace(request.Discount)
	if discount != "" {
		discount, ok = contactdata.NormalizePercent(discount)
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

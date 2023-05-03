package phonenumber

import (
	"errors"
	"fmt"
	"unicode"
)

func Number(phoneNumber string) (string, error) {
	n := ""
	for _, c := range phoneNumber {
		if unicode.IsDigit(c) {
			n += string(c)
		}
	}
	if len(n) == 11 {
		if string(n[0]) == "1" {
			n = n[len(n)-10:]
		}
	}
	if len(n) == 10 && string(n[3]) != "1" && string(n[3]) != "0" && string(n[0]) != "1" && string(n[0]) != "0" {
		return n, nil
	}
	return n, errors.New("phonenumber not valid")
}

func AreaCode(phoneNumber string) (string, error) {
	n, err := Number(phoneNumber)
	return n[:3], err
}

func Format(phoneNumber string) (string, error) {
	n, err := Number(phoneNumber)
	return fmt.Sprintf("(%s) %s-%s", n[:3], n[3:6], n[6:]), err
}

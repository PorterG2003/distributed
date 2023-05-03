package wordy

import (
	"strconv"
	"strings"
)

func Answer(question string) (int, bool) {
	question = strings.TrimSuffix(question, "?")
	questionSplit := strings.Split(question, " ")
	length := len(questionSplit)

	var a int

	if length == 3 {
		n, err := strconv.Atoi(questionSplit[2])
		if err != nil {
			return 0, false
		} else {
			return n, true
		}
	}

	for i := 2; i < length; i++ {
		n, err := strconv.Atoi(questionSplit[i])
		if err == nil {
			n++
			n, err := strconv.Atoi(questionSplit[i-1])
			if err == nil {
				n++
				return 0, false
			}
		}
		if i == 2 {
			n, err := strconv.Atoi(questionSplit[2])
			if err != nil {
				return 0, false
			} else {
				a = n
			}
		} else if questionSplit[i] == "minus" {
			if i+1 >= length {
				return 0, false
			}
			n, err := strconv.Atoi(questionSplit[i+1])
			if err != nil {
				return 0, false
			} else {
				a -= n
			}
		} else if questionSplit[i] == "plus" {
			if i+1 >= length {
				return 0, false
			}
			n, err := strconv.Atoi(questionSplit[i+1])
			if err != nil {
				return 0, false
			} else {
				a += n
			}
		} else if questionSplit[i] == "multiplied" {
			if i+2 >= length {
				return 0, false
			}
			n, err := strconv.Atoi(questionSplit[i+2])
			if err != nil {
				return 0, false
			} else {
				a *= n
			}
		} else if questionSplit[i] == "divided" {
			if i+2 >= length {
				return 0, false
			}
			n, err := strconv.Atoi(questionSplit[i+2])
			if err != nil {
				return 0, false
			} else {
				a /= n
			}
		} else if i == len(questionSplit)-1 {
			n, err := strconv.Atoi(questionSplit[i])
			if err != nil {
				return 0, false
			} else {
				n++
				return a, true
			}
		}
	}
	return 0, false
}

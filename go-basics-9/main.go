package encode

import (
	"fmt"
	"strconv"
	"unicode"
)

func RunLengthEncode(input string) string {
	output := ""
	count := 1
	prev := ""
	input += "9"
	for i, c := range input {
		fmt.Println(prev, string(c))
		if string(c) == prev {
			count++
			continue
		} else if count > 1 {
			output += strconv.Itoa(count) + prev
		} else if count == 1 || i == 0 && input[i] != input[i+1] {
			output += prev
		}
		prev = string(c)
		count = 1
	}
	return output
}

func RunLengthDecode(input string) string {
	input += ""
	output := ""
	prev := 'A'
	numberIndex := 0
	for i, c := range input {
		if (unicode.IsLetter(c) || unicode.IsSpace(c)) && (unicode.IsLetter(prev) || unicode.IsSpace(prev)) {
			output += string(c)
			numberIndex = i + 1
		} else if unicode.IsLetter(c) || unicode.IsSpace(c) {
			fmt.Println(input[numberIndex:i])
			x, err := strconv.Atoi(input[numberIndex:i])
			if err == nil {
				for j := 0; j < x; j++ {
					output += string(c)
				}
			} else {
				fmt.Println("error", err)
			}
			numberIndex = i + 1
		}
		prev = c
	}
	return output
}

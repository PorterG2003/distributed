package diamond

import (
	"errors"
	"fmt"
)

func Gen(char byte) (string, error) {
	if char < 'A' || char > 'Z' {
		return "", errors.New("Char out of range")
	}
	alphMap := map[byte]int{
		'A': 1,
		'B': 2,
		'C': 3,
		'D': 4,
		'E': 5,
		'F': 6,
		'G': 7,
		'H': 8,
		'I': 9,
		'J': 10,
		'K': 11,
		'L': 12,
		'M': 13,
		'N': 14,
		'O': 15,
		'P': 16,
		'Q': 17,
		'R': 18,
		'S': 19,
		'T': 20,
		'U': 21,
		'V': 22,
		'W': 23,
		'X': 24,
		'Y': 25,
		'Z': 26,
	}
	alph := [26]byte{
		'A',
		'B',
		'C',
		'D',
		'E',
		'F',
		'G',
		'H',
		'I',
		'J',
		'K',
		'L',
		'M',
		'N',
		'O',
		'P',
		'Q',
		'R',
		'S',
		'T',
		'U',
		'V',
		'W',
		'X',
		'Y',
		'Z'}

	result := ""
	var prevs []string
	line := ""
	for i := 0; i < alphMap[char]; i++ {
		line = ""
		for j := 0; j < alphMap[char]-i-1; j++ {
			line += " "
		}
		line += string(alph[i])
		for j := 0; j < i*2-1; j++ {
			line += " "
		}
		if i != 0 {
			line += string(alph[i])
		}
		for j := 0; j < alphMap[char]-i-1; j++ {
			line += " "
		}
		line += "\n"
		prevs = append(prevs, line)
		result += line
	}
	for i := len(prevs) - 2; i >= 0; i-- {
		result += prevs[i]
	}
	fmt.Print(result)
	return result, nil
}

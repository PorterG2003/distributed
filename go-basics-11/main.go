package thefarm

import (
	"fmt"
)

// See types.go for the types defined for this exercise.

// TODO: Define the SillyNephewError type here.
type SillyNephewError struct {
	message string
}

func (m *SillyNephewError) Error() string {
	return m.message
}

// DivideFood computes the fodder amount per cow for the given cows.
func DivideFood(weightFodder WeightFodder, cows int) (float64, error) {
	fodder, err := weightFodder.FodderAmount()
	if err == ErrScaleMalfunction && fodder > 0 {
		fodder *= 2
	} else if err != ErrScaleMalfunction && err != nil {
		return 0, err
	}
	if fodder < 0 {
		return 0, &SillyNephewError{
			message: "negative fodder",
		}
	}
	if cows == 0 {
		return 0, &SillyNephewError{
			message: "division by zero",
		}
	}
	if cows < 0 {
		return 0, &SillyNephewError{
			message: fmt.Sprintf("silly nephew, there cannot be %d cows", cows),
		}
	}
	return fodder / float64(cows), nil
}

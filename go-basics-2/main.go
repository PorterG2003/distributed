// Package weather provides your weather needs.
package weather

// CurrentCondition represents the current weather condition for a city.
var CurrentCondition string

// CurrentLocation represents the name of a city.
var CurrentLocation string

// Forecast returns a string showing a city and it's weather condition.
func Forecast(city, condition string) string {
	CurrentLocation, CurrentCondition = city, condition
	return CurrentLocation + " - current weather condition: " + CurrentCondition
}

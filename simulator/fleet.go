package main

import "regexp"

func validEquipment(id string) bool {
	return regexp.MustCompile(`^TEL-CVDTIN-08A-(0[1-9]|10)$`).MatchString(id)
}

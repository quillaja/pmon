package main

import (
	"fmt"
	"strconv"
	"strings"
)

type Size uint64

// Size constants 0-6
const (
	bite Size = iota // 1<<(10*0)
	kilo             // 1<<(10*1) etc...
	mega
	giga
	tera
	peta
	exta
)

// Size units
const (
	B Size = 1 << (10 * iota)
	KiB
	MiB
	GiB
	TiB
	PiB
	EiB
	Default = 0
)

var suffix = [...]string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}

func u(c Size) Size { return 1 << (10 * c) }

func (s Size) String() string {
	for i := bite; i < exta; i++ {
		fit := s / u(i+1)
		if fit == 0 {
			return fmt.Sprintf("%.4g%s", float64(s)/float64(u(i)), suffix[i])
		}
	}
	return fmt.Sprintf("%d%s", s, suffix[exta])
}

func (s Size) Suffix() string {
	for i := bite; i < exta; i++ {
		fit := s / u(i+1)
		if fit == 0 {
			return suffix[i]
		}
	}
	return suffix[exta]
}

func (s Size) In(unit Size) float64 {
	return float64(s) / float64(unit)
}

func (s Size) InString(unit Size) string {
	if unit == 0 {
		return s.String()
	}

	// do some trickery so that when the converted
	// size has all zeros after the decimal place,
	// it will print as an integer instead of with
	// the trailing zeros. If the decimal places are
	// important, print the size as a float.
	in := s.In(unit)
	if float64(s/unit) == in {
		return fmt.Sprintf("%d", int64(in))
	}
	return fmt.Sprintf("%.3f", in)

}

func ParseUnit(unit string) (Size, error) {
	s := strings.ToLower(unit)
	switch s {
	case "":
		return Default, nil
	case "b":
		return B, nil
	case "kb", "kib":
		return KiB, nil
	case "mb", "mib":
		return MiB, nil
	case "gb", "gib":
		return GiB, nil
	case "tb", "tib":
		return TiB, nil
	case "pb", "pib":
		return PiB, nil
	case "eb", "eib":
		return EiB, nil
	}

	return 0, fmt.Errorf("%s invalid size", unit)
}

func ParseSize(s string) (Size, error) {
	suf := strings.TrimSpace(strings.TrimLeft(s, ".-0123456789"))
	num := strings.TrimSpace(strings.TrimSuffix(s, suf))
	n, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid numeral \"%s\" in %s", num, s)
	}
	unit, err := ParseUnit(suf)
	if err != nil {
		return 0, fmt.Errorf("invalid unit in %s: %s", s, err)
	}
	return Size(n * float64(unit)), nil
}

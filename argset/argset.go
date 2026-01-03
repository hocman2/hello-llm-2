package argset

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	argTypeBool = iota
	argTypeInt
	argTypeString
)

type argDef struct {
	retValue any
	argType int
	value any
	short rune
	long string
}

type ArgSet struct {
	args []argDef
	ignoredArgs []string
}

func NewArgSet() ArgSet {
	return ArgSet{}
}

func (a *ArgSet) Args() []string {
	return a.ignoredArgs
}

func (a *ArgSet) AddFlag(retValue *bool, short rune, long string, defaultValue bool) {
	a.args = append(a.args, argDef{retValue: retValue, argType: argTypeBool, value: defaultValue, short: short, long: long})
}

func (a *ArgSet) AddInt(retValue *int, short rune, long string, defaultValue int) {
	a.args = append(a.args, argDef{retValue: retValue, argType: argTypeInt, value: defaultValue, short: short, long: long})
}

func (a *ArgSet) AddString(retValue *string, short rune, long string, defaultValue string) {
	a.args = append(a.args, argDef{retValue: retValue, argType: argTypeString, value: defaultValue, short: short, long: long})
}

func (a *ArgSet) tryFindDef(long string, short rune) *argDef {
	for i, _ := range a.args {
		def := &a.args[i]
		if len(long) > 0 && def.long == long {
			return def
		} else if def.short == short {
			return def
		}
	}

	return nil
}

func readValueInNextArgs(arg string, cursor *int, args []string) (string, error) {
	*cursor += 1
	if args[*cursor] == "=" {
		*cursor += 1
	}
	value := args[*cursor]
	if strings.HasPrefix(value, "-") {
		return "", errors.New(fmt.Sprintf("%s: value can't start with '-' or '--'", arg))
	}
	return value, nil
}

func parseValueToType(arg string, dest any, argType int, value string) error {
	if argType == argTypeInt {
		i, err := strconv.Atoi(value)
		if err != nil {
			return errors.New(fmt.Sprintf("%s expects a numeric value", arg))
		}

		if p, ok := dest.(*int); ok {
			*p = i
		} else {
			panic("Expected dest to be an int pointer")
		}
	} else {
		if p, ok := dest.(*string); ok {
			*p = value
		} else {
			panic("Expected dest to be a string pointer")
		}
	}
	return nil
}

func (a *ArgSet) Parse(args []string) error {
	argCursor := 0
	for argCursor < len(args) {
		arg := args[argCursor]

		if arg[0] == '\\' {
			a.ignoredArgs = append(a.ignoredArgs, args[argCursor])
			argCursor += 1
			continue
		}

		if len(arg) > 2 && arg[:2] == "--" {
			long, _, _ := strings.Cut(arg[2:], "=")
			def := a.tryFindDef(long, '0')
			if def == nil {
				return errors.New(fmt.Sprintf("Unknown argument: %s", arg))
			}

			var value string
			switch def.argType {
			case argTypeBool:
				if p, ok := def.retValue.(*bool); ok {
					*p = !def.value.(bool)
				} else {
					panic("Expected a bool pointer")
				}
			case argTypeInt, argTypeString:
				if _, after, found := strings.Cut(arg, "="); found {
					value = after
				} else {
					var err error
					value, err = readValueInNextArgs(arg, &argCursor, args)
					if err != nil {
						return err
					}
				}

				if err := parseValueToType(arg, def.retValue, def.argType, value); err != nil {
					return err
				}
			}
			argCursor += 1
		} else if len(arg) > 1 && arg[0] == '-' {
			argCut, _, _ := strings.Cut(arg[1:], "=")
			for _, short := range argCut {
				def := a.tryFindDef("", short)
				if def == nil {
					return errors.New(fmt.Sprintf("Unknown argument: -%c", short))
				}

				var value string
				switch def.argType {
				case argTypeBool:
					if p, ok := def.retValue.(*bool); ok {
						*p = !def.value.(bool)
					} else {
						panic("Expected a bool pointer")
					}
				case argTypeInt, argTypeString:
					if before, after, found := strings.Cut(arg, "="); found {
						if len(before) > 2 {
							return errors.New(fmt.Sprintf("Compound arguments cannot be assigned a value: %s\nUse the long version or separate each flag", arg))
						}
						value = after
					} else if len(arg) > 2 {
						return errors.New(fmt.Sprintf("Compound arguments cannot be assigned a value: %s\nUse the long version or separate each flag", arg))
					} else {
						var err error
						value, err = readValueInNextArgs(arg, &argCursor, args)
						if err != nil {
							return err
						}
					}

					if err := parseValueToType(arg, def.retValue, def.argType, value); err != nil {
						return err
					}
				}
			}

			argCursor += 1
		} else {
			a.ignoredArgs = append(a.ignoredArgs, args[argCursor])
			argCursor += 1
			continue
		}
	}

	return nil
}

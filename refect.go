package main

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

func FieldWriter(target interface{}, path string, value string, overWrite bool) (original interface{}, err error) {
	defer func() {
		recoverErr := recover()
		if recoverErr != nil {
			switch x := recoverErr.(type) {
			case string:
				err = errors.New(x)
			case error:
				err = x
			default:
				err = errors.New("unknown error")
			}
		}
		if err != nil {
			DErrorf("Reflect Error | Reflect config error on bad path | path=%s error=%s", path, err.Error())
		}
	}()

	elem := reflect.ValueOf(target).Elem()
	kind := elem.Kind()
	paths := strings.Split(strings.TrimSpace(path), ".")

	for i := 0; i < len(paths); i += 1 {
		currentKey := "Main." + strings.Join(paths[:i+1], ".")

		if elem.Kind() == reflect.Struct {
			if typeof, ok := elem.Type().FieldByName(paths[i]); ok {
				fw := typeof.Tag.Get("fw")
				switch fw {
				case "-":
					return "", errors.New("ERR: this field is protected: " + currentKey)
				case "readonly":
					if overWrite {
						return "", errors.New("ERR: this field is readonly: " + currentKey)
					}
				}
			}
			elem = reflect.Indirect(elem).FieldByName(paths[i])
			kind = elem.Kind()
		} else {
			return "", errors.New("ERR: cannot move to unstructed field: " + currentKey)
		}

		if !elem.IsValid() {
			return "", errors.New("ERR: cannot find field: " + currentKey)
		}
	}

	if overWrite && !elem.CanSet() {
		return "", errors.New("ERR: do not have write access to the value")
	}

	if kind != reflect.String &&
		kind != reflect.Int64 &&
		kind != reflect.Bool {
		return "", fmt.Errorf("ERR: setting the value of [%s]<%s> is not supported yet", path, kind.String())
	}

	if kind == reflect.Int64 {
		original = int64(elem.Int())
		if overWrite {
			i64, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return "", errors.New("ERR: cannot parse " + value + " to " + kind.String())
			}
			elem.Set(reflect.ValueOf(i64))
		}
	} else if kind == reflect.Bool {
		original = elem.Bool()
		if overWrite {
			if ContainsString([]string{"TRUE", "true", "ON", "on", "YES", "yes", "Y", "y", "1"}, value) {
				elem.Set(reflect.ValueOf(true))
			} else if ContainsString([]string{"FALSE", "false", "OFF", "off", "NO", "no", "N", "n", "0"}, value) {
				elem.Set(reflect.ValueOf(false))
			} else {
				return "", errors.New("ERR: cannot parse " + value + " to " + kind.String())
			}

		}
	} else if kind == reflect.String {
		original = elem.String()
		if overWrite {
			elem.Set(reflect.ValueOf(value))
		}
	}

	if !overWrite {
		DLogf("Reflect | Config read path=%s original=%v", path, original)
	} else {
		DLogf("Reflect | Config change path=%s original=%v modified=%v", path, original, value)
	}

	return
}

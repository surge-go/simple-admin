package config

import (
	"reflect"
	"unsafe"
)

type cloneVisit struct {
	typ reflect.Type
	ptr uintptr
	len int
	cap int
}

// cloneConfigValue detaches supported reference values from those held by
// Viper. Structs containing synchronization primitives are returned as-is.
func cloneConfigValue(value any) any {
	if value == nil {
		return nil
	}

	cloned := cloneReflectValue(reflect.ValueOf(value), make(map[cloneVisit]reflect.Value))
	if !cloned.IsValid() {
		return nil
	}
	return cloned.Interface()
}

func cloneReflectValue(value reflect.Value, visited map[cloneVisit]reflect.Value) reflect.Value {
	if !value.IsValid() {
		return value
	}

	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		inner := cloneReflectValue(value.Elem(), visited)
		out := reflect.New(value.Type()).Elem()
		if inner.IsValid() && inner.Type().AssignableTo(value.Type()) {
			out.Set(inner)
		}
		return out
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		if valueContainsSyncPrimitive(value, make(map[cloneVisit]bool)) {
			return value
		}
		visit := cloneVisit{typ: value.Type(), ptr: uintptr(value.UnsafePointer())}
		if cloned, ok := visited[visit]; ok {
			return cloned
		}
		out := reflect.MakeMapWithSize(value.Type(), value.Len())
		visited[visit] = out
		for _, key := range value.MapKeys() {
			clonedKey := cloneReflectValue(key, visited)
			item := cloneReflectValue(value.MapIndex(key), visited)
			out.SetMapIndex(clonedKey, item)
		}
		return out
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		if valueContainsSyncPrimitive(value, make(map[cloneVisit]bool)) {
			return value
		}
		visit := cloneVisit{typ: value.Type(), ptr: value.Pointer(), len: value.Len(), cap: value.Cap()}
		if cloned, ok := visited[visit]; ok {
			return cloned
		}
		out := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		visited[visit] = out
		for i := 0; i < value.Len(); i++ {
			out.Index(i).Set(cloneReflectValue(value.Index(i), visited))
		}
		return out
	case reflect.Ptr:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		if valueContainsSyncPrimitive(value, make(map[cloneVisit]bool)) {
			return value
		}
		visit := cloneVisit{typ: value.Type(), ptr: value.Pointer()}
		if cloned, ok := visited[visit]; ok {
			return cloned
		}
		out := reflect.New(value.Type().Elem())
		visited[visit] = out
		out.Elem().Set(cloneReflectValue(value.Elem(), visited))
		return out
	case reflect.Struct:
		if !value.CanInterface() {
			return value
		}
		if valueContainsSyncPrimitive(value, make(map[cloneVisit]bool)) {
			return value
		}
		out := reflect.New(value.Type()).Elem()
		out.Set(value)
		for i := 0; i < out.NumField(); i++ {
			target := out.Field(i)
			if !target.CanSet() {
				target = reflect.NewAt(target.Type(), unsafe.Pointer(target.UnsafeAddr())).Elem()
			}
			target.Set(cloneReflectValue(target, visited))
		}
		return out
	default:
		return value
	}
}

// Types containing synchronization state are returned without recursive
// cloning. Copying these values cannot provide safe isolation from their
// standard-library invariants.
func typeContainsSyncPrimitive(typ reflect.Type, seen map[reflect.Type]bool) bool {
	if seen[typ] {
		return false
	}
	seen[typ] = true

	if typ.PkgPath() == "sync" {
		switch typ.Name() {
		case "Mutex", "RWMutex", "Once", "WaitGroup", "Cond", "Pool", "Map":
			return true
		}
	}
	if typ.PkgPath() == "sync/atomic" {
		return true
	}
	if typ.Name() == "noCopy" {
		return true
	}

	switch typ.Kind() {
	case reflect.Array, reflect.Pointer, reflect.Slice:
		return typeContainsSyncPrimitive(typ.Elem(), seen)
	case reflect.Map:
		return typeContainsSyncPrimitive(typ.Key(), seen) || typeContainsSyncPrimitive(typ.Elem(), seen)
	case reflect.Struct:
		for i := 0; i < typ.NumField(); i++ {
			if typeContainsSyncPrimitive(typ.Field(i).Type, seen) {
				return true
			}
		}
	}
	return false
}

func valueContainsSyncPrimitive(value reflect.Value, seen map[cloneVisit]bool) bool {
	if !value.IsValid() {
		return false
	}
	if typeContainsSyncPrimitive(value.Type(), make(map[reflect.Type]bool)) {
		return true
	}

	switch value.Kind() {
	case reflect.Interface:
		return !value.IsNil() && valueContainsSyncPrimitive(value.Elem(), seen)
	case reflect.Pointer:
		if value.IsNil() {
			return false
		}
		visit := cloneVisit{typ: value.Type(), ptr: value.Pointer()}
		if seen[visit] {
			return false
		}
		seen[visit] = true
		return valueContainsSyncPrimitive(value.Elem(), seen)
	case reflect.Map:
		if value.IsNil() {
			return false
		}
		visit := cloneVisit{typ: value.Type(), ptr: uintptr(value.UnsafePointer())}
		if seen[visit] {
			return false
		}
		seen[visit] = true
		iter := value.MapRange()
		for iter.Next() {
			if valueContainsSyncPrimitive(iter.Key(), seen) || valueContainsSyncPrimitive(iter.Value(), seen) {
				return true
			}
		}
	case reflect.Slice:
		if value.IsNil() {
			return false
		}
		visit := cloneVisit{typ: value.Type(), ptr: value.Pointer(), len: value.Len(), cap: value.Cap()}
		if seen[visit] {
			return false
		}
		seen[visit] = true
		for i := 0; i < value.Len(); i++ {
			if valueContainsSyncPrimitive(value.Index(i), seen) {
				return true
			}
		}
	case reflect.Array:
		for i := 0; i < value.Len(); i++ {
			if valueContainsSyncPrimitive(value.Index(i), seen) {
				return true
			}
		}
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if valueContainsSyncPrimitive(value.Field(i), seen) {
				return true
			}
		}
	}
	return false
}

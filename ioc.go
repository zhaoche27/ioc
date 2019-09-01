package ioc

import (
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"time"
)

type InjectScope int

const (
	SingletonScope InjectScope = iota
	PrototypeScope
)

type Object struct {
	name         string
	value        interface{}
	reflectType  reflect.Type
	reflectValue reflect.Value
	embedded     bool
}

type Inject struct {
	sync.Mutex
	isLock  bool
	named   map[string]*Object
	unnamed map[reflect.Type]*Object
}

func (inject *Inject) provide(o *Object) error {
	if !inject.isLock {
		inject.Lock()
		defer func() {
			inject.Unlock()
			inject.isLock = false
		}()
		inject.isLock = true
	}
	if o.name == "" {
		if !isStructPtr(o.reflectType) {
			return fmt.Errorf(
				"expected unnamed object value to be a pointer to a struct but got type %s "+
					"with value %v",
				o.reflectType,
				o.value,
			)
		}
		if inject.unnamed == nil {
			inject.unnamed = make(map[reflect.Type]*Object)
		}
		if _, ok := inject.unnamed[o.reflectType]; ok {
			return nil
		}
		inject.unnamed[o.reflectType] = o
	} else {
		if inject.named == nil {
			inject.named = make(map[string]*Object)
		}
		if _, ok := inject.named[o.name]; ok {
			return nil
		}
		inject.named[o.name] = o
	}
	return nil
}

func (inject *Inject) Provide(value interface{}) error {
	return inject.ProvideByName("", value)
}

func (inject *Inject) ProvideByName(name string, value interface{}) error {
	reflectType, reflectValue := reflect.TypeOf(value), reflect.ValueOf(value)
	return inject.provide(&Object{name: name, value: value, reflectType: reflectType, reflectValue: reflectValue})
}

func (inject *Inject) Instance(value interface{}) (interface{}, error) {
	return inject.InstanceByScope(value, SingletonScope)
}

func (inject *Inject) InstanceByScope(value interface{}, scope InjectScope) (interface{}, error) {
	inject.Lock()
	defer func() {
		inject.Unlock()
		inject.isLock = false
	}()
	inject.isLock = true
	reflectType := reflect.TypeOf(value)
	o, ok := inject.unnamed[reflectType]
	if !ok {
		if !isStructPtr(reflectType) {
			err := fmt.Errorf(
				"expected unnamed object value to be a pointer to a struct but got type %s "+
					"with value %v",
				reflectType,
				value,
			)
			return nil, err
		}
		o = &Object{value: value, reflectType: reflectType, reflectValue: reflect.ValueOf(value)}
		err := inject.populateExplicit(o)
		if err != nil {
			return nil, err
		}
		err = inject.provide(o)
		if err != nil {
			return nil, err
		}
	}
	return inject.instanceByScope(o, scope)
}

func (inject *Inject) instanceByScope(o *Object, scope InjectScope) (interface{}, error) {
	if scope == SingletonScope {
		return o.value, nil
	}
	dst := Copy(o.value)
	return dst, nil
}

func (inject *Inject) Objects() []*Object {
	objects := make([]*Object, 0, len(inject.unnamed)+len(inject.named))
	for _, o := range inject.unnamed {
		if !o.embedded {
			objects = append(objects, o)
		}
	}
	for _, o := range inject.named {
		if !o.embedded {
			objects = append(objects, o)
		}
	}
	// randomize to prevent callers from relying on ordering
	for i := 0; i < len(objects); i++ {
		j := rand.Intn(i + 1)
		objects[i], objects[j] = objects[j], objects[i]
	}
	return objects
}

func (inject *Inject) populateExplicit(o *Object) error {
	reflectValue := reflect.Indirect(o.reflectValue)
	for i := 0; i < reflectValue.NumField(); i++ {
		fieldValue := reflectValue.Field(i)
		fieldType := fieldValue.Type()
		field := o.reflectType.Elem().Field(i)
		fieldTag := field.Tag
		fieldName := field.Name
		tag, err := parseTag(string(fieldTag))
		if err != nil {
			err = fmt.Errorf(
				"unexpected tag format `%s` for field %s in type %s",
				string(fieldTag),
				fieldName,
				o.reflectType,
			)
			return err
		}
		if tag == nil {
			continue
		}
		if !reflectValue.Field(i).CanSet() {
			err = fmt.Errorf(
				"inject requested on unexported field %s in type %s",
				fieldName,
				o.reflectType,
			)
			return err
		}
		if !isNilOrZero(fieldValue, fieldType) {
			continue
		}
		if tag.name != "" {
			existing := inject.named[tag.name]
			if existing == nil {
				err = fmt.Errorf(
					"did not find object named %s required by field %s in type %s",
					tag.name,
					fieldName,
					o.reflectType,
				)
				return err
			}
			if !existing.reflectType.AssignableTo(fieldType) {
				err = fmt.Errorf(
					"object named %s of type %s is not assignable to field %s (%s) in type %s",
					tag.name,
					fieldType,
					fieldName,
					existing.reflectType,
					o.reflectType,
				)
				return err
			}
			fieldValue.Set(existing.reflectValue)
			continue
		}
		if fieldType.Kind() == reflect.Interface {
			var found *Object
			for _, existing := range inject.unnamed {
				if existing.reflectType.AssignableTo(fieldType) {
					if found != nil {
						return fmt.Errorf(
							"found two assignable values for field %s in type %s. one type "+
								"%s with value %v and another type %s with value %v",
							fieldName,
							o.reflectType,
							found.reflectType,
							found.value,
							existing.reflectType,
							existing.reflectValue,
						)
					}
					found = existing
				}
			}
			if found == nil {
				err = fmt.Errorf(
					"not found assignable values for field %s in type %s",
					fieldName,
					o.reflectType,
				)
				return err
			}
			fieldValue.Set(found.reflectValue)
			continue
		}
		existing := inject.unnamed[fieldType]
		if existing == nil {
			if !isStructPtr(fieldType) {
				err = fmt.Errorf(
					"expected unnamed object value to be a pointer to a struct but got type %s "+
						"with value %v",
					o.reflectType,
					fieldValue.Interface(),
				)
				return err
			}
			newValue := reflect.New(fieldType.Elem()).Interface()
			oFiled := &Object{value: newValue, reflectType: fieldType, reflectValue: reflect.ValueOf(newValue),
				embedded: field.Anonymous}
			err := inject.populateExplicit(oFiled)
			if err != nil {
				return err
			}
			err = inject.provide(oFiled)
			if err != nil {
				return err
			}
			existing = oFiled
		}
		fieldValue.Set(existing.reflectValue)
		continue
	}
	return nil
}

type injector interface {
	Provide(value interface{}) error
	ProvideByName(name string, value interface{}) error
	Instance(value interface{}) (interface{}, error)
	InstanceByScope(value interface{}, scope InjectScope) (interface{}, error)
}

type tag struct {
	name string
}

func parseTag(t string) (*tag, error) {
	found, value, err := Extract("inject", t)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	return &tag{name: value}, nil
}

func isStructPtr(t reflect.Type) bool {
	return t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Struct
}

func isNilOrZero(v reflect.Value, t reflect.Type) bool {
	switch v.Kind() {
	default:
		return reflect.DeepEqual(v.Interface(), reflect.Zero(t).Interface())
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
}

// copier for delegating copy process to type
type copier interface {
	DeepCopy() interface{}
}

// Iface is an alias to Copy; this exists for backwards compatibility reasons.
func Iface(iface interface{}) interface{} {
	return Copy(iface)
}

// Copy creates a deep copy of whatever is passed to it and returns the copy
// in an interface{}.  The returned value will need to be asserted to the
// correct type.
func Copy(src interface{}) interface{} {
	if src == nil {
		return nil
	}
	original := reflect.ValueOf(src)
	cpy := reflect.New(original.Type()).Elem()
	copyRecursive(original, cpy)
	return cpy.Interface()
}

// copyRecursive does the actual copying of the interface. It currently has
// limited support for what it can handle. Add as needed.
func copyRecursive(original, cpy reflect.Value) {
	// check for implement deepcopy.copier
	if original.CanInterface() {
		if copier, ok := original.Interface().(copier); ok {
			cpy.Set(reflect.ValueOf(copier.DeepCopy()))
			return
		}
	}

	// handle according to original's Kind
	switch original.Kind() {
	case reflect.Ptr:
		// Get the actual value being pointed to.
		originalValue := original.Elem()

		// if  it isn't valid, return.
		if !originalValue.IsValid() {
			return
		}
		cpy.Set(reflect.New(originalValue.Type()))
		copyRecursive(originalValue, cpy.Elem())

	case reflect.Interface:
		// If this is a nil, don't do anything
		if original.IsNil() {
			return
		}
		// Get the value for the interface, not the pointer.
		originalValue := original.Elem()

		// Get the value by calling Elem().
		copyValue := reflect.New(originalValue.Type()).Elem()
		copyRecursive(originalValue, copyValue)
		cpy.Set(copyValue)

	case reflect.Struct:
		t, ok := original.Interface().(time.Time)
		if ok {
			cpy.Set(reflect.ValueOf(t))
			return
		}
		// Go through each field of the struct and copy it.
		for i := 0; i < original.NumField(); i++ {
			if original.Field(i).CanSet() {
				continue
			}
			copyRecursive(original.Field(i), cpy.Field(i))
		}

	case reflect.Slice:
		if original.IsNil() {
			return
		}
		// Make a new slice and copy each element.
		cpy.Set(reflect.MakeSlice(original.Type(), original.Len(), original.Cap()))
		for i := 0; i < original.Len(); i++ {
			copyRecursive(original.Index(i), cpy.Index(i))
		}

	case reflect.Map:
		if original.IsNil() {
			return
		}
		cpy.Set(reflect.MakeMap(original.Type()))
		for _, key := range original.MapKeys() {
			originalValue := original.MapIndex(key)
			copyValue := reflect.New(originalValue.Type()).Elem()
			copyRecursive(originalValue, copyValue)
			copyKey := Copy(key.Interface())
			cpy.SetMapIndex(reflect.ValueOf(copyKey), copyValue)
		}

	default:
		cpy.Set(original)
	}
}

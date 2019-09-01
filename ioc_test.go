package ioc

import (
	"fmt"
	"testing"
)

type User struct {
}

func (u User) String() string {
	return fmt.Sprintf("AAAAAAAA=%T", u)
}

type Product struct {
	name string
}

func (p *Product) String() string {
	return fmt.Sprintf("=====%T,%p", p, p)
}

func (o *Product) Copy() interface{} {
	fmt.Printf("-----------------------%p\n", o)
	p := &Product{}
	fmt.Printf("-----------------------%p\n", p)
	return p
}

type Order struct {
	U User         `inject:"u"`
	P fmt.Stringer `inject:""`
}

func (o Order) String() string {
	return fmt.Sprint(o.P, o.U)
}

func Test_ioc(t *testing.T) {
	inject := &Inject{}
	err := inject.ProvideByName("u", User{})
	if err != nil {
		t.Error(err)
		return
	}
	err = inject.Provide(&Product{})
	if err != nil {
		t.Error(err)
		return
	}
	o, err := inject.Instance(&Order{})
	if err != nil {
		t.Error(err)
		return
	}
	order := o.(*Order)
	t.Logf("%p", order)
	t.Logf("%v", order.U)
	t.Logf("%p", order.P)
	t.Log(order)
	o, err = inject.InstanceByScope(&Order{}, PrototypeScope)
	if err != nil {
		t.Error(err)
		return
	}
	t.Log("============================================")
	order = o.(*Order)
	t.Logf("%p", order)
	t.Logf("%v", order.U)
	t.Logf("%p", order.P)
	t.Log(order)
}

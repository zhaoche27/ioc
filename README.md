# IOC
> Inversion of Control，控制反转，实现依赖注入

> 接口 injector

```go
type injector interface 
    // 提供实例
	Provide(value interface{}) error
    // 提供有名实例
	ProvideByName(name string, value interface{}) error
    // 获取实例（单例）
	Instance(value interface{}) (interface{}, error)
    // 获取实例
	InstanceByScope(value interface{}, scope InjectScope) (interface{}, error)
}
```

```go
type Order struct {
	U User         `inject:"u"`  //注入名称为`u`的User实例
	P fmt.Stringer `inject:""`   //注入实现接口Stringer的实例
}
```
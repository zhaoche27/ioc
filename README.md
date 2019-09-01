# IOC
> Inversion of Control，控制反转，实现依赖注入

> 接口 injector

```go
type injector interface 
    // 注册实例
	Provide(value interface{}) error
    // 注册有名实例
	ProvideByName(name string, value interface{}) error
    // 获取实例（单例）
	Instance(value interface{}) (interface{}, error)
    // 获取实例
	InstanceByScope(value interface{}, scope InjectScope) (interface{}, error)
}
```
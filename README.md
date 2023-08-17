- #### 虚拟设备通过UDP进行通信，需要在初始化时指定虚拟设备的MAC地址、IP地址以及UDP端口号

- #### 如果要使用虚拟设备测试近场通信的全部功能，需要至少创建两个虚拟设备并分别运行在两个终端，例如：

  ```go
  func StartDeviceA() {
  	ctx, _ := context.WithCancel(context.Background())
  	strategy := nfd.Strategy{0}
  	configCh := make(chan nfd.ConfigInfo)
  	deviceCh := make(chan nfd.DeviceInfo)
  	n := nfd.NewNearFieldDevice("192.168.0.1", "", "0013a20041bb76a4", "localhost", 8001)
  	n.Init(strategy)
  	n.Run(ctx, configCh, deviceCh)
  }
  
  func main() {
  	_, err := log.NewLogger()
  	if err != nil {
  		fmt.Printf("%v\n", err)
  	}
  	go StartDeviceA()
  	for {
  	}
  }
  ```

  ```go
  func StartDeviceB() {
  	ctx, _ := context.WithCancel(context.Background())
  	strategy := nfd.Strategy{0}
  	configCh := make(chan nfd.ConfigInfo)
  	deviceCh := make(chan nfd.DeviceInfo)
  	n := nfd.NewNearFieldDevice("192.168.0.2", "", "0013a20041bb7684", "localhost", 8002)
  	n.Init(strategy)
  	n.Run(ctx, configCh, deviceCh)
  }
  
  func main() {
  	_, err := log.NewLogger()
  	if err != nil {
  		fmt.Printf("%v\n", err)
  	}
  	go StartDeviceB()
  	for {
  	}
  }
  ```

  
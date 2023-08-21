- #### 虚拟设备通过UDP进行通信，需要在初始化时指定虚拟设备的MAC地址、IP地址以及UDP端口号

- #### 如果要使用虚拟设备测试近场通信的全部功能，需要创建两个虚拟设备并分别运行在两个终端，例如：

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

  此外，还需将 nfd/virtual_xbee/virtual_xbee_device.go 中的 peer 修改为对端的MAC地址。

  data.go中模拟了一个网络层报文，如果需要使用data.go中的数据进行测试，需要修改报文中对应的源和目的IP地址。
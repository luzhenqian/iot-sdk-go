package device

import (
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"iot-sdk-go/pkg/types"
	protocol "iot-sdk-go/sdk/protocol"
	request "iot-sdk-go/sdk/request"
	serializer "iot-sdk-go/sdk/serializer"
	storage "iot-sdk-go/sdk/storage"
	topics "iot-sdk-go/sdk/topics"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/imdario/mergo"
)

// Device 设备
type Device struct {
	ProductKey string
	Name       string
	Version    string
	Secret     string
	ID         int64
	Token      []byte
	Access     string
	Protocol   protocol.Protocol
	Serializer serializer.Serializer
	Topics     topics.Topics
	Storage    storage.Storage
}

// GetDeviceInfo 获取设备信息
func (d *Device) GetDeviceInfo() (*Device, error) {
	ProductKeyInter, err := d.Storage.Get("ProductKey")
	if err != nil {
		return nil, err
	}
	ProductKey, _ := types.InterfaceToString(ProductKeyInter)
	NameInter, err := d.Storage.Get("Name")
	if err != nil {
		return nil, err
	}
	Name, _ := types.InterfaceToString(NameInter)
	SecretInter, err := d.Storage.Get("Secret")
	if err != nil {
		return nil, err
	}
	Secret, _ := types.InterfaceToString(SecretInter)
	VersionInter, err := d.Storage.Get("Version")
	if err != nil {
		return nil, err
	}
	Version, _ := types.InterfaceToString(VersionInter)
	IDInter, err := d.Storage.Get("ID")
	if err != nil {
		return nil, err
	}
	IDInt, _ := types.InterfaceToInt(IDInter)
	ID := int64(IDInt)
	return &Device{
		ProductKey: ProductKey,
		Name:       Name,
		Secret:     Secret,
		Version:    Version,
		ID:         ID,
	}, nil
}

// LoadDeviceInfo 合并设备信息
func (d *Device) LoadDeviceInfo() error {
	tmp, err := d.GetDeviceInfo()
	if err != nil {
		return err
	}
	return mergo.Merge(d, tmp, mergo.WithOverride)
}

// SetDeviceInfo 设置设备信息
func (d *Device) SetDeviceInfo() error {
	storage := d.Storage
	if d.ProductKey != "" {
		if err := storage.Set("ProductKey", d.ProductKey); err != nil {
			return err
		}
	}
	if d.Name != "" {
		if err := storage.Set("Name", d.Name); err != nil {
			return err
		}
	}
	if d.Secret != "" {
		if err := storage.Set("Secret", d.Secret); err != nil {
			return err
		}
	}
	if d.Version != "" {
		if err := storage.Set("Version", d.Version); err != nil {
			return err
		}
	}
	if d.ID != 0 {
		if err := storage.Set("ID", d.ID); err != nil {
			return err
		}
	}
	if d.Token != nil {
		if err := storage.Set("Token", d.Token); err != nil {
			return err
		}
	}
	if d.Access != "" {
		if err := storage.Set("Access", d.Access); err != nil {
			return err
		}
	}
	return nil
}

// Register 注册
func (d *Device) Register() error {
	args, err := RegisterArgsFromDevice(*d)
	if err != nil {
		return err
	}
	argsStr, err := json.Marshal(args)
	if err != nil {
		return err
	}
	jsonresp, err := http.Post(d.Topics.Register, "application/json", strings.NewReader(string(argsStr)))
	if err != nil {
		return err
	}
	response := RegisterResponse{}
	body, _ := ioutil.ReadAll(jsonresp.Body)
	err = json.Unmarshal(body, &response)
	if err != nil {
		return err
	}
	if err := HTTPIsOK(response); err != nil {
		return err
	}
	d.ID = response.Data.ID
	d.Secret = response.Data.Secret
	d.SetDeviceInfo()
	return nil
}

// Login 登陆
func (d *Device) Login() error {
	args, err := AuthArgsFromDevice(*d)
	if err != nil {
		return err
	}
	argsStr, err := json.Marshal(args)
	if err != nil {
		return err
	}
	jsonresp, err := http.Post(d.Topics.Login, "application/json", strings.NewReader(string(argsStr)))
	if err != nil {
		return err
	}
	response := AuthResponse{}
	body, _ := ioutil.ReadAll(jsonresp.Body)
	err = json.Unmarshal(body, &response)
	if err != nil {
		return err
	}
	if err := HTTPIsOK(response); err != nil {
		return err
	}
	hexToken, err := hex.DecodeString(response.Data.AccessToken)
	if err != nil {
		return err
	}
	d.Token = hexToken
	d.Access = response.Data.AccessAddr
	d.SetDeviceInfo()
	return nil
}

// AutoLogin 自动登录
func (d *Device) AutoLogin() error {
	if d.Token == nil || d.Access == "" {
		if err := d.Register(); err != nil {
			return err
		}
	}
	return d.Login()
}

// InitProtocolClient 初始化协议客户端
func (d *Device) InitProtocolClient(opts ...interface{}) error {
	if len(opts) > 0 {
		// 用户传入配置，使用配置创建客户端
		return d.Protocol.NewClient(opts[0])
	}
	// 默认创建 MQTT 配置
	IDStr := strconv.Itoa(int(d.ID))
	TokenStr := hex.EncodeToString(d.Token) // 817aecf06c023365
	mqttOpts := map[string]interface{}{
		"Broker":    d.Access,
		"ClientID":  IDStr,
		"Username":  IDStr,
		"Password":  TokenStr,
		"KeepAlive": 30 * time.Second,
	}
	newOpts := d.Protocol.MakeOpts(mqttOpts)
	return d.Protocol.NewClient(newOpts)
}

// Publish 发布
func (d *Device) Publish(request request.Request) error {
	params := protocol.OptionsFormatter(request)
	return d.Protocol.Publish(params)
}

// Subscribe 订阅
func (d *Device) Subscribe(request request.Request) error {
	opts := protocol.OptionsFormatter(request)
	return d.Protocol.Subscribe(opts)
}

// Unsubscribe 取消订阅
func (d *Device) Unsubscribe(topics []string) error {
	return d.Protocol.Unsubscribe(map[string]interface{}{"topics": topics})
}

// toSerializerProperty device.Property 转换到 serializer.Property
func (p *Property) toSerializerProperty() *serializer.Property {
	sp := &serializer.Property{}
	sp.PropertyID = p.PropertyID
	sp.SubDeviceID = p.SubDeviceID
	sp.Value = p.Value
	return sp
}

// PostProperty 上报属性
func (d *Device) PostProperty(property Property) error {
	data, err := d.Serializer.MakePostPropertyData(property.toSerializerProperty())
	if err != nil {
		return err
	}
	request := protocol.OptionsFormatter(*makePostPropertyRequest(d, data))
	if err != nil {
		return err
	}
	return d.Protocol.Publish(request)
}

// makePostPropertyRequest 创建属性请求
func makePostPropertyRequest(d *Device, payload []byte) *request.Request {
	request := &request.Request{}
	request.Topic = d.Topics.PostProperty
	request.Qos = 1
	request.Retained = false
	request.Payload = payload
	return request
}

// InitOptions 初始化配置项
type InitOptions struct {
	AutoReregister               bool
	AutoRelogin                  bool
	AutoReInitProtocolClient     bool
	ReregisterInterval           time.Duration
	ReloginInterval              time.Duration
	ReInitProtocolClientInterval time.Duration
}

var defaultInitOptions = InitOptions{
	AutoReregister:               false,
	AutoRelogin:                  false,
	AutoReInitProtocolClient:     false,
	ReregisterInterval:           5 * time.Second,
	ReloginInterval:              5 * time.Second,
	ReInitProtocolClientInterval: 5 * time.Second,
}

func getFinallyInitOpts(opts ...InitOptions) InitOptions {
	finallyOpts := defaultInitOptions
	if len(opts) > 0 {
		finallyOpts = opts[0]
	}
	return finallyOpts
}

// AutoInit 自动初始化
func (d *Device) AutoInit(opts ...InitOptions) error {
	finallyOpts := getFinallyInitOpts(opts...)
	if types.IsNil(d.Protocol.GetInstance()) {
		if err := d.AutoLogin(); err != nil {
			if finallyOpts.AutoRelogin {
				for {
					time.Sleep(finallyOpts.ReregisterInterval)
					if err := d.AutoLogin(); err == nil {
						break
					}
				}
			} else {
				return err
			}
		}
		if err := d.InitProtocolClient(); err != nil {
			if finallyOpts.AutoReInitProtocolClient {
				for {
					time.Sleep(finallyOpts.ReInitProtocolClientInterval)
					if err := d.InitProtocolClient(); err == nil {
						break
					}
				}
			} else {
				return err
			}
		}
	}
	return nil
}

// AutoPostProperty 自动上报属性
func (d *Device) AutoPostProperty(property Property, opts ...InitOptions) error {
	finallyOpts := getFinallyInitOpts(opts...)
	if types.IsNil(d.Protocol.GetInstance()) {
		d.AutoInit(finallyOpts)
	}
	return d.PostProperty(property)
}

// OnProperty 设置属性
func (d *Device) OnProperty(callback func(property interface{})) {

}

// PostEvent 发送事件
func (d *Device) PostEvent(identifier string, property []interface{}) error {
	request := request.Request{}
	request.Topic = d.Topics.PostEvent
	request.Qos = 1
	payload, err := d.Serializer.Marshal(property)
	request.Payload = payload
	if err != nil {
		return err
	}
	params := protocol.OptionsFormatter(request)
	d.Protocol.Publish(params)
	return nil
}

// Command 命令
type Command struct {
	ID       uint16
	Callback func(map[int]interface{})
}

// OnCommand 响应命令
func (d *Device) OnCommand(cmds ...Command) error {
	callbacks := make(map[uint16]func(map[int]interface{}))
	for _, cmd := range cmds {
		callbacks[cmd.ID] = cmd.Callback
	}
	callbackFn := func(resp request.Response) {
		p := resp.Payload()
		cmdPayload, _ := d.Serializer.UnmarshalCommand(p)
		params := cmdPayload.Params
		params[-1] = cmdPayload.SubDeviceID
		if callback, ok := callbacks[cmdPayload.ID]; ok {
			callback(params)
		}
	}
	return d.Protocol.Subscribe(protocol.OptionsFormatter(*makeOnCommandRequest(d, callbackFn)))
}

func makeOnCommandRequest(d *Device, callbackFn func(resp request.Response)) *request.Request {
	r := &request.Request{}
	r.Topic = d.Topics.OnCommand
	r.Qos = 1
	r.Callback = callbackFn
	return r
}
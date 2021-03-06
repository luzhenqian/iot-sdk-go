package device

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"iot-sdk-go/pkg/typeconv"
	"iot-sdk-go/sdk/httpclient"
	"iot-sdk-go/sdk/protocol"
	"iot-sdk-go/sdk/request"
	"iot-sdk-go/sdk/serializer"
	"iot-sdk-go/sdk/storage"
	"iot-sdk-go/sdk/topics"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/imdario/mergo"
	"github.com/pkg/errors"
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
	HTTPClient http.Client
}

// Option 配置函数
type Option func(*Device)

// New 创建设备
func New(ProductKey, Name, Version string, opts ...func(*Device)) *Device {
	device := &Device{
		ProductKey: ProductKey,
		Name:       Name,
		Version:    Version,
		Protocol:   protocol.NewMQTT(),
		Serializer: serializer.NewTLV(),
		Topics:     topics.DefaultTopics,
		Storage:    &storage.LocalStorage{},
		HTTPClient: httpclient.DefaultClient,
	}
	for _, opt := range opts {
		opt(device)
	}
	return device
}

// Protocol 设置协议
func Protocol(protocol protocol.Protocol) Option {
	return func(d *Device) {
		d.Protocol = protocol
	}
}

// Serializer 设置序列化器
func Serializer(serializer serializer.Serializer) Option {
	return func(d *Device) {
		d.Serializer = serializer
	}
}

// Topics 设置主题列表
func Topics(topics topics.Topics) Option {
	return func(d *Device) {
		d.Topics = topics
	}
}

// Storage 设置存储
func Storage(storage storage.Storage) Option {
	return func(d *Device) {
		d.Storage = storage
	}
}

// HTTPClient 设置 Http 客户端
func HTTPClient(HTTPClient http.Client) Option {
	return func(d *Device) {
		d.HTTPClient = HTTPClient
	}
}

// GetDeviceInfo 获取设备信息
func (d *Device) GetDeviceInfo() (*Device, error) {
	ProductKeyInter, err := d.Storage.Get(d.Name + ".ProductKey")
	if err != nil {
		return nil, err
	}
	ProductKey, _ := typeconv.InterfaceToString(ProductKeyInter)

	NameInter, err := d.Storage.Get(d.Name + ".Name")
	if err != nil {
		return nil, err
	}
	Name, _ := typeconv.InterfaceToString(NameInter)

	SecretInter, err := d.Storage.Get(d.Name + ".Secret")
	if err != nil {
		return nil, err
	}
	Secret, _ := typeconv.InterfaceToString(SecretInter)

	VersionInter, err := d.Storage.Get(d.Name + ".Version")
	if err != nil {
		return nil, err
	}
	Version, _ := typeconv.InterfaceToString(VersionInter)

	IDInter, err := d.Storage.Get(d.Name + ".ID")
	if err != nil {
		return nil, err
	}
	IDInt, _ := typeconv.InterfaceToInt(IDInter)
	ID := int64(IDInt)

	AccessInter, err := d.Storage.Get(d.Name + ".Access")
	if err != nil {
		return nil, err
	}
	Access, _ := typeconv.InterfaceToString(AccessInter)

	TokenInter, err := d.Storage.Get(d.Name + ".Token")
	if err != nil {
		return nil, err
	}
	Token, _ := typeconv.InterfaceToSliceByte(TokenInter)

	return &Device{
		ProductKey: ProductKey,
		Name:       Name,
		Secret:     Secret,
		Version:    Version,
		ID:         ID,
		Access:     Access,
		Token:      Token,
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
		if err := storage.Set(d.Name+".ProductKey", d.ProductKey); err != nil {
			return err
		}
	}
	if d.Name != "" {
		if err := storage.Set(d.Name+".Name", d.Name); err != nil {
			return err
		}
	}
	if d.Secret != "" {
		if err := storage.Set(d.Name+".Secret", d.Secret); err != nil {
			return err
		}
	}
	if d.Version != "" {
		if err := storage.Set(d.Name+".Version", d.Version); err != nil {
			return err
		}
	}
	if d.ID != 0 {
		if err := storage.Set(d.Name+".ID", d.ID); err != nil {
			return err
		}
	}
	if d.Token != nil {
		if err := storage.Set(d.Name+".Token", d.Token); err != nil {
			return err
		}
	}
	if d.Access != "" {
		if err := storage.Set(d.Name+".Access", d.Access); err != nil {
			return err
		}
	}
	return nil
}

// Register 注册
func (d *Device) Register() error {
	args, err := RegisterArgsFromDevice(*d)
	if err != nil {
		return errors.Wrap(err, "device register failed, from device create register arguments failed")
	}
	argsStr, err := json.Marshal(args)
	if err != nil {
		return errors.Wrap(err, "device register failed, register arguments convert to json failed")
	}
	jsonresp, err := d.HTTPClient.Post(d.Topics.Register, "application/json", strings.NewReader(string(argsStr)))
	if err != nil {
		return errors.Wrap(err, "device register failed, register response is error")
	}
	response := RegisterResponse{}
	body, _ := ioutil.ReadAll(jsonresp.Body)
	err = json.Unmarshal(body, &response)
	if err != nil {
		return errors.Wrap(err, "device register failed, register rest api response convert to json failed")
	}
	if err := HTTPIsOK(response); err != nil {
		return errors.Wrap(err, "device register failed, register rest api state not is ok")
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
		return errors.Wrap(err, "device login failed, from device create auth arguments failed")
	}
	argsStr, err := json.Marshal(args)
	if err != nil {
		return errors.Wrap(err, "device login failed, auth arguments convert to json failed")
	}
	jsonresp, err := d.HTTPClient.Post(d.Topics.Login, "application/json", strings.NewReader(string(argsStr)))
	if err != nil {
		return errors.Wrap(err, "device login failed, request login rest api failed")
	}
	response := AuthResponse{}
	body, _ := ioutil.ReadAll(jsonresp.Body)
	err = json.Unmarshal(body, &response)
	if err != nil {
		return errors.Wrap(err, "device login failed, login rest api response convert to json failed")
	}
	if err := HTTPIsOK(response); err != nil {
		return errors.Wrap(err, "device login failed, login rest api state not is ok")
	}
	hexToken, err := hex.DecodeString(response.Data.AccessToken)
	if err != nil {
		return errors.Wrap(err, "device login failed, access convert to byte failed")
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
	return d.initMQTTClient()
}

func (d *Device) initMQTTClient() error {
	IDStr := strconv.Itoa(int(d.ID))
	TokenStr := hex.EncodeToString(d.Token) // 817aecf06c023365
	mqttOpts := map[string]interface{}{
		"Broker":    d.Access,
		"ClientID":  IDStr,
		"Username":  IDStr,
		"Password":  TokenStr,
		"KeepAlive": 30 * time.Second,
		// 断开后，执行 login，刷新 token，重连
		"OnConnectionLost": func() map[string]interface{} {
			fmt.Println("connection lost")
			d.Login()
			return map[string]interface{}{
				"Password": d.Token,
			}
		},
	}
	newOpts, err := d.Protocol.MakeOpts(mqttOpts)
	if err != nil {
		return errors.Wrap(err, "init mqtt client failed")
	}
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
	data, err := d.Serializer.MakePropertyData(property.toSerializerProperty())
	if err != nil {
		return err
	}
	request := protocol.OptionsFormatter(*makePostPropertyRequest(d, data))
	if err != nil {
		return err
	}
	return d.Protocol.Publish(request)
}

// makePostPropertyRequest 创建上报属性请求
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
	if typeconv.IsNil(d.Protocol.GetInstance()) {
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
	if typeconv.IsNil(d.Protocol.GetInstance()) {
		if err := d.AutoInit(finallyOpts); err != nil {
			return errors.Wrap(err, "auto init failed")
		}
	}
	return d.PostProperty(property)
}

// OnProperty 设置属性
func (d *Device) OnProperty(callback func(property interface{})) {

}

// PostEvent 发送事件
func (d *Device) PostEvent(identifier string, property Property) error {
	data, err := d.Serializer.MakeEventData(property.toSerializerProperty())
	if err != nil {
		return err
	}
	request := protocol.OptionsFormatter(*makePostEventRequest(d, data))
	return d.Protocol.Publish(request)
}

// makePostEventRequest 创建上报事件请求
func makePostEventRequest(d *Device, payload []byte) *request.Request {
	request := &request.Request{}
	request.Topic = d.Topics.PostEvent
	request.Qos = 1
	request.Retained = false
	request.Payload = payload
	return request
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
		cmdPayload, err := d.Serializer.UnmarshalCommand(p)
		if err != nil {
			// TODO log
			return
		}
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

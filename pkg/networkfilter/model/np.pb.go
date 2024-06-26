// Code generated by protoc-gen-go. DO NOT EDIT.
// source: np.proto

//option go_package="./;ncsnp";

package ncsnp

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

// ***************************************使用场景：1、资源管理通过sync推送控制器；2、控制器通过SBI下发设备；3、UI查询 **********************************************************
type VLinkSla struct {
	Delay                uint32   `protobuf:"varint,1,opt,name=Delay,proto3" json:"Delay,omitempty"`
	Jitter               uint32   `protobuf:"varint,2,opt,name=Jitter,proto3" json:"Jitter,omitempty"`
	Loss                 uint32   `protobuf:"varint,3,opt,name=Loss,proto3" json:"Loss,omitempty"`
	Bandwidth            uint64   `protobuf:"varint,4,opt,name=Bandwidth,proto3" json:"Bandwidth,omitempty"`
	FreeBandwidth        uint64   `protobuf:"varint,5,opt,name=FreeBandwidth,proto3" json:"FreeBandwidth,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *VLinkSla) Reset()         { *m = VLinkSla{} }
func (m *VLinkSla) String() string { return proto.CompactTextString(m) }
func (*VLinkSla) ProtoMessage()    {}
func (*VLinkSla) Descriptor() ([]byte, []int) {
	return fileDescriptor_223620259f5885fd, []int{0}
}

func (m *VLinkSla) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_VLinkSla.Unmarshal(m, b)
}
func (m *VLinkSla) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_VLinkSla.Marshal(b, m, deterministic)
}
func (m *VLinkSla) XXX_Merge(src proto.Message) {
	xxx_messageInfo_VLinkSla.Merge(m, src)
}
func (m *VLinkSla) XXX_Size() int {
	return xxx_messageInfo_VLinkSla.Size(m)
}
func (m *VLinkSla) XXX_DiscardUnknown() {
	xxx_messageInfo_VLinkSla.DiscardUnknown(m)
}

var xxx_messageInfo_VLinkSla proto.InternalMessageInfo

func (m *VLinkSla) GetDelay() uint32 {
	if m != nil {
		return m.Delay
	}
	return 0
}

func (m *VLinkSla) GetJitter() uint32 {
	if m != nil {
		return m.Jitter
	}
	return 0
}

func (m *VLinkSla) GetLoss() uint32 {
	if m != nil {
		return m.Loss
	}
	return 0
}

func (m *VLinkSla) GetBandwidth() uint64 {
	if m != nil {
		return m.Bandwidth
	}
	return 0
}

func (m *VLinkSla) GetFreeBandwidth() uint64 {
	if m != nil {
		return m.FreeBandwidth
	}
	return 0
}

type FieldVLink struct {
	LocalNodeSN          string    `protobuf:"bytes,1,opt,name=LocalNodeSN,proto3" json:"LocalNodeSN,omitempty"`
	RemoteNodeSN         string    `protobuf:"bytes,2,opt,name=RemoteNodeSN,proto3" json:"RemoteNodeSN,omitempty"`
	LocalInterface       string    `protobuf:"bytes,3,opt,name=LocalInterface,proto3" json:"LocalInterface,omitempty"`
	AttachId             uint64    `protobuf:"varint,4,opt,name=AttachId,proto3" json:"AttachId,omitempty"`
	VLinkSlaAttr         *VLinkSla `protobuf:"bytes,5,opt,name=VLinkSlaAttr,proto3" json:"VLinkSlaAttr,omitempty"`
	OpaqueValue          string    `protobuf:"bytes,6,opt,name=OpaqueValue,proto3" json:"OpaqueValue,omitempty"`
	XXX_NoUnkeyedLiteral struct{}  `json:"-"`
	XXX_unrecognized     []byte    `json:"-"`
	XXX_sizecache        int32     `json:"-"`
}

func (m *FieldVLink) Reset()         { *m = FieldVLink{} }
func (m *FieldVLink) String() string { return proto.CompactTextString(m) }
func (*FieldVLink) ProtoMessage()    {}
func (*FieldVLink) Descriptor() ([]byte, []int) {
	return fileDescriptor_223620259f5885fd, []int{1}
}

func (m *FieldVLink) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_FieldVLink.Unmarshal(m, b)
}
func (m *FieldVLink) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_FieldVLink.Marshal(b, m, deterministic)
}
func (m *FieldVLink) XXX_Merge(src proto.Message) {
	xxx_messageInfo_FieldVLink.Merge(m, src)
}
func (m *FieldVLink) XXX_Size() int {
	return xxx_messageInfo_FieldVLink.Size(m)
}
func (m *FieldVLink) XXX_DiscardUnknown() {
	xxx_messageInfo_FieldVLink.DiscardUnknown(m)
}

var xxx_messageInfo_FieldVLink proto.InternalMessageInfo

func (m *FieldVLink) GetLocalNodeSN() string {
	if m != nil {
		return m.LocalNodeSN
	}
	return ""
}

func (m *FieldVLink) GetRemoteNodeSN() string {
	if m != nil {
		return m.RemoteNodeSN
	}
	return ""
}

func (m *FieldVLink) GetLocalInterface() string {
	if m != nil {
		return m.LocalInterface
	}
	return ""
}

func (m *FieldVLink) GetAttachId() uint64 {
	if m != nil {
		return m.AttachId
	}
	return 0
}

func (m *FieldVLink) GetVLinkSlaAttr() *VLinkSla {
	if m != nil {
		return m.VLinkSlaAttr
	}
	return nil
}

func (m *FieldVLink) GetOpaqueValue() string {
	if m != nil {
		return m.OpaqueValue
	}
	return ""
}

type DomainVLink struct {
	LocalDomainName      string    `protobuf:"bytes,1,opt,name=LocalDomainName,proto3" json:"LocalDomainName,omitempty"`
	LocalDomainId        uint32    `protobuf:"varint,2,opt,name=LocalDomainId,proto3" json:"LocalDomainId,omitempty"`
	RemoteDomainName     string    `protobuf:"bytes,3,opt,name=RemoteDomainName,proto3" json:"RemoteDomainName,omitempty"`
	RemoteDomainId       uint32    `protobuf:"varint,4,opt,name=RemoteDomainId,proto3" json:"RemoteDomainId,omitempty"`
	LocalNodeSN          string    `protobuf:"bytes,5,opt,name=LocalNodeSN,proto3" json:"LocalNodeSN,omitempty"`
	RemoteNodeSN         string    `protobuf:"bytes,6,opt,name=RemoteNodeSN,proto3" json:"RemoteNodeSN,omitempty"`
	LocalInterface       string    `protobuf:"bytes,7,opt,name=LocalInterface,proto3" json:"LocalInterface,omitempty"`
	AttachDomainId       uint64    `protobuf:"varint,8,opt,name=AttachDomainId,proto3" json:"AttachDomainId,omitempty"`
	VLinkSlaAttr         *VLinkSla `protobuf:"bytes,9,opt,name=VLinkSlaAttr,proto3" json:"VLinkSlaAttr,omitempty"`
	OpaqueValue          string    `protobuf:"bytes,10,opt,name=OpaqueValue,proto3" json:"OpaqueValue,omitempty"`
	AttachDomainName     string    `protobuf:"bytes,11,opt,name=AttachDomainName,proto3" json:"AttachDomainName,omitempty"`
	IngressPeerSN        string    `protobuf:"bytes,12,opt,name=IngressPeerSN,proto3" json:"IngressPeerSN,omitempty"`
	EgressPeerSN         string    `protobuf:"bytes,13,opt,name=EgressPeerSN,proto3" json:"EgressPeerSN,omitempty"`
	Isp                  string    `protobuf:"bytes,14,opt,name=Isp,proto3" json:"Isp,omitempty"`
	XXX_NoUnkeyedLiteral struct{}  `json:"-"`
	XXX_unrecognized     []byte    `json:"-"`
	XXX_sizecache        int32     `json:"-"`
}

func (m *DomainVLink) Reset()         { *m = DomainVLink{} }
func (m *DomainVLink) String() string { return proto.CompactTextString(m) }
func (*DomainVLink) ProtoMessage()    {}
func (*DomainVLink) Descriptor() ([]byte, []int) {
	return fileDescriptor_223620259f5885fd, []int{2}
}

func (m *DomainVLink) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_DomainVLink.Unmarshal(m, b)
}
func (m *DomainVLink) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_DomainVLink.Marshal(b, m, deterministic)
}
func (m *DomainVLink) XXX_Merge(src proto.Message) {
	xxx_messageInfo_DomainVLink.Merge(m, src)
}
func (m *DomainVLink) XXX_Size() int {
	return xxx_messageInfo_DomainVLink.Size(m)
}
func (m *DomainVLink) XXX_DiscardUnknown() {
	xxx_messageInfo_DomainVLink.DiscardUnknown(m)
}

var xxx_messageInfo_DomainVLink proto.InternalMessageInfo

func (m *DomainVLink) GetLocalDomainName() string {
	if m != nil {
		return m.LocalDomainName
	}
	return ""
}

func (m *DomainVLink) GetLocalDomainId() uint32 {
	if m != nil {
		return m.LocalDomainId
	}
	return 0
}

func (m *DomainVLink) GetRemoteDomainName() string {
	if m != nil {
		return m.RemoteDomainName
	}
	return ""
}

func (m *DomainVLink) GetRemoteDomainId() uint32 {
	if m != nil {
		return m.RemoteDomainId
	}
	return 0
}

func (m *DomainVLink) GetLocalNodeSN() string {
	if m != nil {
		return m.LocalNodeSN
	}
	return ""
}

func (m *DomainVLink) GetRemoteNodeSN() string {
	if m != nil {
		return m.RemoteNodeSN
	}
	return ""
}

func (m *DomainVLink) GetLocalInterface() string {
	if m != nil {
		return m.LocalInterface
	}
	return ""
}

func (m *DomainVLink) GetAttachDomainId() uint64 {
	if m != nil {
		return m.AttachDomainId
	}
	return 0
}

func (m *DomainVLink) GetVLinkSlaAttr() *VLinkSla {
	if m != nil {
		return m.VLinkSlaAttr
	}
	return nil
}

func (m *DomainVLink) GetOpaqueValue() string {
	if m != nil {
		return m.OpaqueValue
	}
	return ""
}

func (m *DomainVLink) GetAttachDomainName() string {
	if m != nil {
		return m.AttachDomainName
	}
	return ""
}

func (m *DomainVLink) GetIngressPeerSN() string {
	if m != nil {
		return m.IngressPeerSN
	}
	return ""
}

func (m *DomainVLink) GetEgressPeerSN() string {
	if m != nil {
		return m.EgressPeerSN
	}
	return ""
}

func (m *DomainVLink) GetIsp() string {
	if m != nil {
		return m.Isp
	}
	return ""
}

type FieldTopoCacheNotify struct {
	SequenceNum          uint64        `protobuf:"varint,1,opt,name=SequenceNum,proto3" json:"SequenceNum,omitempty"`
	LocalDomainId        uint32        `protobuf:"varint,2,opt,name=LocalDomainId,proto3" json:"LocalDomainId,omitempty"`
	LocalDomainName      string        `protobuf:"bytes,3,opt,name=LocalDomainName,proto3" json:"LocalDomainName,omitempty"`
	LocalNodeSN          string        `protobuf:"bytes,4,opt,name=LocalNodeSN,proto3" json:"LocalNodeSN,omitempty"`
	FieldVLinkArray      []*FieldVLink `protobuf:"bytes,5,rep,name=FieldVLinkArray,proto3" json:"FieldVLinkArray,omitempty"`
	XXX_NoUnkeyedLiteral struct{}      `json:"-"`
	XXX_unrecognized     []byte        `json:"-"`
	XXX_sizecache        int32         `json:"-"`
}

func (m *FieldTopoCacheNotify) Reset()         { *m = FieldTopoCacheNotify{} }
func (m *FieldTopoCacheNotify) String() string { return proto.CompactTextString(m) }
func (*FieldTopoCacheNotify) ProtoMessage()    {}
func (*FieldTopoCacheNotify) Descriptor() ([]byte, []int) {
	return fileDescriptor_223620259f5885fd, []int{3}
}

func (m *FieldTopoCacheNotify) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_FieldTopoCacheNotify.Unmarshal(m, b)
}
func (m *FieldTopoCacheNotify) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_FieldTopoCacheNotify.Marshal(b, m, deterministic)
}
func (m *FieldTopoCacheNotify) XXX_Merge(src proto.Message) {
	xxx_messageInfo_FieldTopoCacheNotify.Merge(m, src)
}
func (m *FieldTopoCacheNotify) XXX_Size() int {
	return xxx_messageInfo_FieldTopoCacheNotify.Size(m)
}
func (m *FieldTopoCacheNotify) XXX_DiscardUnknown() {
	xxx_messageInfo_FieldTopoCacheNotify.DiscardUnknown(m)
}

var xxx_messageInfo_FieldTopoCacheNotify proto.InternalMessageInfo

func (m *FieldTopoCacheNotify) GetSequenceNum() uint64 {
	if m != nil {
		return m.SequenceNum
	}
	return 0
}

func (m *FieldTopoCacheNotify) GetLocalDomainId() uint32 {
	if m != nil {
		return m.LocalDomainId
	}
	return 0
}

func (m *FieldTopoCacheNotify) GetLocalDomainName() string {
	if m != nil {
		return m.LocalDomainName
	}
	return ""
}

func (m *FieldTopoCacheNotify) GetLocalNodeSN() string {
	if m != nil {
		return m.LocalNodeSN
	}
	return ""
}

func (m *FieldTopoCacheNotify) GetFieldVLinkArray() []*FieldVLink {
	if m != nil {
		return m.FieldVLinkArray
	}
	return nil
}

type DomainTopoCacheNotify struct {
	SequenceNum          uint64         `protobuf:"varint,1,opt,name=SequenceNum,proto3" json:"SequenceNum,omitempty"`
	LocalDomainId        uint32         `protobuf:"varint,2,opt,name=LocalDomainId,proto3" json:"LocalDomainId,omitempty"`
	LocalDomainName      string         `protobuf:"bytes,3,opt,name=LocalDomainName,proto3" json:"LocalDomainName,omitempty"`
	LocalNodeSN          string         `protobuf:"bytes,4,opt,name=LocalNodeSN,proto3" json:"LocalNodeSN,omitempty"`
	DomainVLinkArray     []*DomainVLink `protobuf:"bytes,5,rep,name=DomainVLinkArray,proto3" json:"DomainVLinkArray,omitempty"`
	XXX_NoUnkeyedLiteral struct{}       `json:"-"`
	XXX_unrecognized     []byte         `json:"-"`
	XXX_sizecache        int32          `json:"-"`
}

func (m *DomainTopoCacheNotify) Reset()         { *m = DomainTopoCacheNotify{} }
func (m *DomainTopoCacheNotify) String() string { return proto.CompactTextString(m) }
func (*DomainTopoCacheNotify) ProtoMessage()    {}
func (*DomainTopoCacheNotify) Descriptor() ([]byte, []int) {
	return fileDescriptor_223620259f5885fd, []int{4}
}

func (m *DomainTopoCacheNotify) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_DomainTopoCacheNotify.Unmarshal(m, b)
}
func (m *DomainTopoCacheNotify) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_DomainTopoCacheNotify.Marshal(b, m, deterministic)
}
func (m *DomainTopoCacheNotify) XXX_Merge(src proto.Message) {
	xxx_messageInfo_DomainTopoCacheNotify.Merge(m, src)
}
func (m *DomainTopoCacheNotify) XXX_Size() int {
	return xxx_messageInfo_DomainTopoCacheNotify.Size(m)
}
func (m *DomainTopoCacheNotify) XXX_DiscardUnknown() {
	xxx_messageInfo_DomainTopoCacheNotify.DiscardUnknown(m)
}

var xxx_messageInfo_DomainTopoCacheNotify proto.InternalMessageInfo

func (m *DomainTopoCacheNotify) GetSequenceNum() uint64 {
	if m != nil {
		return m.SequenceNum
	}
	return 0
}

func (m *DomainTopoCacheNotify) GetLocalDomainId() uint32 {
	if m != nil {
		return m.LocalDomainId
	}
	return 0
}

func (m *DomainTopoCacheNotify) GetLocalDomainName() string {
	if m != nil {
		return m.LocalDomainName
	}
	return ""
}

func (m *DomainTopoCacheNotify) GetLocalNodeSN() string {
	if m != nil {
		return m.LocalNodeSN
	}
	return ""
}

func (m *DomainTopoCacheNotify) GetDomainVLinkArray() []*DomainVLink {
	if m != nil {
		return m.DomainVLinkArray
	}
	return nil
}

type Id2Name struct {
	DomainID             string   `protobuf:"bytes,1,opt,name=DomainID,proto3" json:"DomainID,omitempty"`
	Name                 string   `protobuf:"bytes,2,opt,name=Name,proto3" json:"Name,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Id2Name) Reset()         { *m = Id2Name{} }
func (m *Id2Name) String() string { return proto.CompactTextString(m) }
func (*Id2Name) ProtoMessage()    {}
func (*Id2Name) Descriptor() ([]byte, []int) {
	return fileDescriptor_223620259f5885fd, []int{5}
}

func (m *Id2Name) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Id2Name.Unmarshal(m, b)
}
func (m *Id2Name) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Id2Name.Marshal(b, m, deterministic)
}
func (m *Id2Name) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Id2Name.Merge(m, src)
}
func (m *Id2Name) XXX_Size() int {
	return xxx_messageInfo_Id2Name.Size(m)
}
func (m *Id2Name) XXX_DiscardUnknown() {
	xxx_messageInfo_Id2Name.DiscardUnknown(m)
}

var xxx_messageInfo_Id2Name proto.InternalMessageInfo

func (m *Id2Name) GetDomainID() string {
	if m != nil {
		return m.DomainID
	}
	return ""
}

func (m *Id2Name) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

type TopoWithFabricMap struct {
	NameMap              []*Id2Name `protobuf:"bytes,1,rep,name=NameMap,proto3" json:"NameMap,omitempty"`
	Contend              []byte     `protobuf:"bytes,2,opt,name=Contend,proto3" json:"Contend,omitempty"`
	XXX_NoUnkeyedLiteral struct{}   `json:"-"`
	XXX_unrecognized     []byte     `json:"-"`
	XXX_sizecache        int32      `json:"-"`
}

func (m *TopoWithFabricMap) Reset()         { *m = TopoWithFabricMap{} }
func (m *TopoWithFabricMap) String() string { return proto.CompactTextString(m) }
func (*TopoWithFabricMap) ProtoMessage()    {}
func (*TopoWithFabricMap) Descriptor() ([]byte, []int) {
	return fileDescriptor_223620259f5885fd, []int{6}
}

func (m *TopoWithFabricMap) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_TopoWithFabricMap.Unmarshal(m, b)
}
func (m *TopoWithFabricMap) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_TopoWithFabricMap.Marshal(b, m, deterministic)
}
func (m *TopoWithFabricMap) XXX_Merge(src proto.Message) {
	xxx_messageInfo_TopoWithFabricMap.Merge(m, src)
}
func (m *TopoWithFabricMap) XXX_Size() int {
	return xxx_messageInfo_TopoWithFabricMap.Size(m)
}
func (m *TopoWithFabricMap) XXX_DiscardUnknown() {
	xxx_messageInfo_TopoWithFabricMap.DiscardUnknown(m)
}

var xxx_messageInfo_TopoWithFabricMap proto.InternalMessageInfo

func (m *TopoWithFabricMap) GetNameMap() []*Id2Name {
	if m != nil {
		return m.NameMap
	}
	return nil
}

func (m *TopoWithFabricMap) GetContend() []byte {
	if m != nil {
		return m.Contend
	}
	return nil
}

// 一种resourcebing的方案
type BindingSelectedDomainPath struct {
	SelectedDomainPath   []*AppConnectSelectedDomainPath `protobuf:"bytes,1,rep,name=SelectedDomainPath,proto3" json:"SelectedDomainPath,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                        `json:"-"`
	XXX_unrecognized     []byte                          `json:"-"`
	XXX_sizecache        int32                           `json:"-"`
}

func (m *BindingSelectedDomainPath) Reset()         { *m = BindingSelectedDomainPath{} }
func (m *BindingSelectedDomainPath) String() string { return proto.CompactTextString(m) }
func (*BindingSelectedDomainPath) ProtoMessage()    {}
func (*BindingSelectedDomainPath) Descriptor() ([]byte, []int) {
	return fileDescriptor_223620259f5885fd, []int{7}
}

func (m *BindingSelectedDomainPath) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_BindingSelectedDomainPath.Unmarshal(m, b)
}
func (m *BindingSelectedDomainPath) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_BindingSelectedDomainPath.Marshal(b, m, deterministic)
}
func (m *BindingSelectedDomainPath) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BindingSelectedDomainPath.Merge(m, src)
}
func (m *BindingSelectedDomainPath) XXX_Size() int {
	return xxx_messageInfo_BindingSelectedDomainPath.Size(m)
}
func (m *BindingSelectedDomainPath) XXX_DiscardUnknown() {
	xxx_messageInfo_BindingSelectedDomainPath.DiscardUnknown(m)
}

var xxx_messageInfo_BindingSelectedDomainPath proto.InternalMessageInfo

func (m *BindingSelectedDomainPath) GetSelectedDomainPath() []*AppConnectSelectedDomainPath {
	if m != nil {
		return m.SelectedDomainPath
	}
	return nil
}

// InterCommunication只选取一个副本的DomainSrPathArray
type AppConnectSelectedDomainPath struct {
	AppConnect           *AppConnectAttr `protobuf:"bytes,1,opt,name=AppConnect,proto3" json:"AppConnect,omitempty"`
	DomainList           []*DomainInfo   `protobuf:"bytes,2,rep,name=DomainList,proto3" json:"DomainList,omitempty"`
	Content              []byte          `protobuf:"bytes,3,opt,name=Content,proto3" json:"Content,omitempty"`
	XXX_NoUnkeyedLiteral struct{}        `json:"-"`
	XXX_unrecognized     []byte          `json:"-"`
	XXX_sizecache        int32           `json:"-"`
}

func (m *AppConnectSelectedDomainPath) Reset()         { *m = AppConnectSelectedDomainPath{} }
func (m *AppConnectSelectedDomainPath) String() string { return proto.CompactTextString(m) }
func (*AppConnectSelectedDomainPath) ProtoMessage()    {}
func (*AppConnectSelectedDomainPath) Descriptor() ([]byte, []int) {
	return fileDescriptor_223620259f5885fd, []int{8}
}

func (m *AppConnectSelectedDomainPath) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_AppConnectSelectedDomainPath.Unmarshal(m, b)
}
func (m *AppConnectSelectedDomainPath) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_AppConnectSelectedDomainPath.Marshal(b, m, deterministic)
}
func (m *AppConnectSelectedDomainPath) XXX_Merge(src proto.Message) {
	xxx_messageInfo_AppConnectSelectedDomainPath.Merge(m, src)
}
func (m *AppConnectSelectedDomainPath) XXX_Size() int {
	return xxx_messageInfo_AppConnectSelectedDomainPath.Size(m)
}
func (m *AppConnectSelectedDomainPath) XXX_DiscardUnknown() {
	xxx_messageInfo_AppConnectSelectedDomainPath.DiscardUnknown(m)
}

var xxx_messageInfo_AppConnectSelectedDomainPath proto.InternalMessageInfo

func (m *AppConnectSelectedDomainPath) GetAppConnect() *AppConnectAttr {
	if m != nil {
		return m.AppConnect
	}
	return nil
}

func (m *AppConnectSelectedDomainPath) GetDomainList() []*DomainInfo {
	if m != nil {
		return m.DomainList
	}
	return nil
}

func (m *AppConnectSelectedDomainPath) GetContent() []byte {
	if m != nil {
		return m.Content
	}
	return nil
}

type KVAttribute struct {
	Key                  string   `protobuf:"bytes,1,opt,name=Key,proto3" json:"Key,omitempty"`
	Value                string   `protobuf:"bytes,2,opt,name=Value,proto3" json:"Value,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *KVAttribute) Reset()         { *m = KVAttribute{} }
func (m *KVAttribute) String() string { return proto.CompactTextString(m) }
func (*KVAttribute) ProtoMessage()    {}
func (*KVAttribute) Descriptor() ([]byte, []int) {
	return fileDescriptor_223620259f5885fd, []int{9}
}

func (m *KVAttribute) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_KVAttribute.Unmarshal(m, b)
}
func (m *KVAttribute) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_KVAttribute.Marshal(b, m, deterministic)
}
func (m *KVAttribute) XXX_Merge(src proto.Message) {
	xxx_messageInfo_KVAttribute.Merge(m, src)
}
func (m *KVAttribute) XXX_Size() int {
	return xxx_messageInfo_KVAttribute.Size(m)
}
func (m *KVAttribute) XXX_DiscardUnknown() {
	xxx_messageInfo_KVAttribute.DiscardUnknown(m)
}

var xxx_messageInfo_KVAttribute proto.InternalMessageInfo

func (m *KVAttribute) GetKey() string {
	if m != nil {
		return m.Key
	}
	return ""
}

func (m *KVAttribute) GetValue() string {
	if m != nil {
		return m.Value
	}
	return ""
}

type AppConnectAttr struct {
	Key                  *AppConnectKey `protobuf:"bytes,1,opt,name=Key,proto3" json:"Key,omitempty"`
	SlaAttr              *AppSlaAttr    `protobuf:"bytes,2,opt,name=SlaAttr,proto3" json:"SlaAttr,omitempty"`
	SrcScnidKVList       []*KVAttribute `protobuf:"bytes,3,rep,name=SrcScnidKVList,proto3" json:"SrcScnidKVList,omitempty"`
	DestScnidKVList      []*KVAttribute `protobuf:"bytes,4,rep,name=DestScnidKVList,proto3" json:"DestScnidKVList,omitempty"`
	Accelerate           bool           `protobuf:"varint,5,opt,name=Accelerate,proto3" json:"Accelerate,omitempty"`
	XXX_NoUnkeyedLiteral struct{}       `json:"-"`
	XXX_unrecognized     []byte         `json:"-"`
	XXX_sizecache        int32          `json:"-"`
}

func (m *AppConnectAttr) Reset()         { *m = AppConnectAttr{} }
func (m *AppConnectAttr) String() string { return proto.CompactTextString(m) }
func (*AppConnectAttr) ProtoMessage()    {}
func (*AppConnectAttr) Descriptor() ([]byte, []int) {
	return fileDescriptor_223620259f5885fd, []int{10}
}

func (m *AppConnectAttr) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_AppConnectAttr.Unmarshal(m, b)
}
func (m *AppConnectAttr) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_AppConnectAttr.Marshal(b, m, deterministic)
}
func (m *AppConnectAttr) XXX_Merge(src proto.Message) {
	xxx_messageInfo_AppConnectAttr.Merge(m, src)
}
func (m *AppConnectAttr) XXX_Size() int {
	return xxx_messageInfo_AppConnectAttr.Size(m)
}
func (m *AppConnectAttr) XXX_DiscardUnknown() {
	xxx_messageInfo_AppConnectAttr.DiscardUnknown(m)
}

var xxx_messageInfo_AppConnectAttr proto.InternalMessageInfo

func (m *AppConnectAttr) GetKey() *AppConnectKey {
	if m != nil {
		return m.Key
	}
	return nil
}

func (m *AppConnectAttr) GetSlaAttr() *AppSlaAttr {
	if m != nil {
		return m.SlaAttr
	}
	return nil
}

func (m *AppConnectAttr) GetSrcScnidKVList() []*KVAttribute {
	if m != nil {
		return m.SrcScnidKVList
	}
	return nil
}

func (m *AppConnectAttr) GetDestScnidKVList() []*KVAttribute {
	if m != nil {
		return m.DestScnidKVList
	}
	return nil
}

func (m *AppConnectAttr) GetAccelerate() bool {
	if m != nil {
		return m.Accelerate
	}
	return false
}

type AppConnectKey struct {
	SrcSCNID             string   `protobuf:"bytes,1,opt,name=SrcSCNID,proto3" json:"SrcSCNID,omitempty"`
	DstSCNID             string   `protobuf:"bytes,2,opt,name=DstSCNID,proto3" json:"DstSCNID,omitempty"`
	SrcID                uint32   `protobuf:"varint,3,opt,name=SrcID,proto3" json:"SrcID,omitempty"`
	DstID                uint32   `protobuf:"varint,4,opt,name=DstID,proto3" json:"DstID,omitempty"`
	SrcDomainId          uint32   `protobuf:"varint,5,opt,name=SrcDomainId,proto3" json:"SrcDomainId,omitempty"`
	DstDomainId          uint32   `protobuf:"varint,6,opt,name=DstDomainId,proto3" json:"DstDomainId,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *AppConnectKey) Reset()         { *m = AppConnectKey{} }
func (m *AppConnectKey) String() string { return proto.CompactTextString(m) }
func (*AppConnectKey) ProtoMessage()    {}
func (*AppConnectKey) Descriptor() ([]byte, []int) {
	return fileDescriptor_223620259f5885fd, []int{11}
}

func (m *AppConnectKey) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_AppConnectKey.Unmarshal(m, b)
}
func (m *AppConnectKey) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_AppConnectKey.Marshal(b, m, deterministic)
}
func (m *AppConnectKey) XXX_Merge(src proto.Message) {
	xxx_messageInfo_AppConnectKey.Merge(m, src)
}
func (m *AppConnectKey) XXX_Size() int {
	return xxx_messageInfo_AppConnectKey.Size(m)
}
func (m *AppConnectKey) XXX_DiscardUnknown() {
	xxx_messageInfo_AppConnectKey.DiscardUnknown(m)
}

var xxx_messageInfo_AppConnectKey proto.InternalMessageInfo

func (m *AppConnectKey) GetSrcSCNID() string {
	if m != nil {
		return m.SrcSCNID
	}
	return ""
}

func (m *AppConnectKey) GetDstSCNID() string {
	if m != nil {
		return m.DstSCNID
	}
	return ""
}

func (m *AppConnectKey) GetSrcID() uint32 {
	if m != nil {
		return m.SrcID
	}
	return 0
}

func (m *AppConnectKey) GetDstID() uint32 {
	if m != nil {
		return m.DstID
	}
	return 0
}

func (m *AppConnectKey) GetSrcDomainId() uint32 {
	if m != nil {
		return m.SrcDomainId
	}
	return 0
}

func (m *AppConnectKey) GetDstDomainId() uint32 {
	if m != nil {
		return m.DstDomainId
	}
	return 0
}

type AppSlaAttr struct {
	DelayValue           uint32   `protobuf:"varint,1,opt,name=DelayValue,proto3" json:"DelayValue,omitempty"`
	LostValue            uint32   `protobuf:"varint,2,opt,name=LostValue,proto3" json:"LostValue,omitempty"`
	JitterValue          uint32   `protobuf:"varint,3,opt,name=JitterValue,proto3" json:"JitterValue,omitempty"`
	ThroughputValue      uint64   `protobuf:"varint,4,opt,name=ThroughputValue,proto3" json:"ThroughputValue,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *AppSlaAttr) Reset()         { *m = AppSlaAttr{} }
func (m *AppSlaAttr) String() string { return proto.CompactTextString(m) }
func (*AppSlaAttr) ProtoMessage()    {}
func (*AppSlaAttr) Descriptor() ([]byte, []int) {
	return fileDescriptor_223620259f5885fd, []int{12}
}

func (m *AppSlaAttr) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_AppSlaAttr.Unmarshal(m, b)
}
func (m *AppSlaAttr) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_AppSlaAttr.Marshal(b, m, deterministic)
}
func (m *AppSlaAttr) XXX_Merge(src proto.Message) {
	xxx_messageInfo_AppSlaAttr.Merge(m, src)
}
func (m *AppSlaAttr) XXX_Size() int {
	return xxx_messageInfo_AppSlaAttr.Size(m)
}
func (m *AppSlaAttr) XXX_DiscardUnknown() {
	xxx_messageInfo_AppSlaAttr.DiscardUnknown(m)
}

var xxx_messageInfo_AppSlaAttr proto.InternalMessageInfo

func (m *AppSlaAttr) GetDelayValue() uint32 {
	if m != nil {
		return m.DelayValue
	}
	return 0
}

func (m *AppSlaAttr) GetLostValue() uint32 {
	if m != nil {
		return m.LostValue
	}
	return 0
}

func (m *AppSlaAttr) GetJitterValue() uint32 {
	if m != nil {
		return m.JitterValue
	}
	return 0
}

func (m *AppSlaAttr) GetThroughputValue() uint64 {
	if m != nil {
		return m.ThroughputValue
	}
	return 0
}

type DomainInfo struct {
	DomainName           string   `protobuf:"bytes,1,opt,name=DomainName,proto3" json:"DomainName,omitempty"`
	DomainId             uint32   `protobuf:"varint,2,opt,name=DomainId,proto3" json:"DomainId,omitempty"`
	DomainType           uint32   `protobuf:"varint,3,opt,name=DomainType,proto3" json:"DomainType,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *DomainInfo) Reset()         { *m = DomainInfo{} }
func (m *DomainInfo) String() string { return proto.CompactTextString(m) }
func (*DomainInfo) ProtoMessage()    {}
func (*DomainInfo) Descriptor() ([]byte, []int) {
	return fileDescriptor_223620259f5885fd, []int{13}
}

func (m *DomainInfo) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_DomainInfo.Unmarshal(m, b)
}
func (m *DomainInfo) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_DomainInfo.Marshal(b, m, deterministic)
}
func (m *DomainInfo) XXX_Merge(src proto.Message) {
	xxx_messageInfo_DomainInfo.Merge(m, src)
}
func (m *DomainInfo) XXX_Size() int {
	return xxx_messageInfo_DomainInfo.Size(m)
}
func (m *DomainInfo) XXX_DiscardUnknown() {
	xxx_messageInfo_DomainInfo.DiscardUnknown(m)
}

var xxx_messageInfo_DomainInfo proto.InternalMessageInfo

func (m *DomainInfo) GetDomainName() string {
	if m != nil {
		return m.DomainName
	}
	return ""
}

func (m *DomainInfo) GetDomainId() uint32 {
	if m != nil {
		return m.DomainId
	}
	return 0
}

func (m *DomainInfo) GetDomainType() uint32 {
	if m != nil {
		return m.DomainType
	}
	return 0
}

func init() {
	proto.RegisterType((*VLinkSla)(nil), "ncsnp.VLinkSla")
	proto.RegisterType((*FieldVLink)(nil), "ncsnp.FieldVLink")
	proto.RegisterType((*DomainVLink)(nil), "ncsnp.DomainVLink")
	proto.RegisterType((*FieldTopoCacheNotify)(nil), "ncsnp.FieldTopoCacheNotify")
	proto.RegisterType((*DomainTopoCacheNotify)(nil), "ncsnp.DomainTopoCacheNotify")
	proto.RegisterType((*Id2Name)(nil), "ncsnp.Id2Name")
	proto.RegisterType((*TopoWithFabricMap)(nil), "ncsnp.TopoWithFabricMap")
	proto.RegisterType((*BindingSelectedDomainPath)(nil), "ncsnp.BindingSelectedDomainPath")
	proto.RegisterType((*AppConnectSelectedDomainPath)(nil), "ncsnp.AppConnectSelectedDomainPath")
	proto.RegisterType((*KVAttribute)(nil), "ncsnp.KVAttribute")
	proto.RegisterType((*AppConnectAttr)(nil), "ncsnp.AppConnectAttr")
	proto.RegisterType((*AppConnectKey)(nil), "ncsnp.AppConnectKey")
	proto.RegisterType((*AppSlaAttr)(nil), "ncsnp.AppSlaAttr")
	proto.RegisterType((*DomainInfo)(nil), "ncsnp.DomainInfo")
}

func init() { proto.RegisterFile("np.proto", fileDescriptor_223620259f5885fd) }

var fileDescriptor_223620259f5885fd = []byte{
	// 938 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xd4, 0x56, 0xcd, 0x6e, 0xe3, 0x36,
	0x10, 0x86, 0xfc, 0x9f, 0x71, 0xec, 0x24, 0x44, 0xb6, 0x50, 0x17, 0x8b, 0x45, 0xa0, 0x16, 0x81,
	0xd1, 0x02, 0x01, 0xea, 0xc5, 0x1e, 0xfa, 0x83, 0x02, 0xde, 0xb8, 0x01, 0xdc, 0x78, 0xdd, 0x05,
	0x15, 0x64, 0xcf, 0x8c, 0xc4, 0x44, 0x42, 0x1d, 0x4a, 0x2b, 0xd1, 0x28, 0xfc, 0x16, 0x3d, 0xf5,
	0x01, 0x7a, 0xe8, 0x2b, 0xf4, 0x91, 0x7a, 0x28, 0xfa, 0x0c, 0x2d, 0x38, 0xa4, 0x2c, 0x4a, 0x72,
	0xdb, 0x45, 0x6f, 0xbd, 0x69, 0xbe, 0xf9, 0x48, 0xce, 0x7c, 0x33, 0xa4, 0x06, 0x06, 0x22, 0xbd,
	0x48, 0xb3, 0x44, 0x26, 0xa4, 0x2b, 0x82, 0x5c, 0xa4, 0xde, 0x8f, 0x0e, 0x0c, 0x6e, 0x97, 0xb1,
	0xf8, 0xde, 0x5f, 0x33, 0x72, 0x0a, 0xdd, 0x39, 0x5f, 0xb3, 0xad, 0xeb, 0x9c, 0x39, 0x93, 0x11,
	0xd5, 0x06, 0xf9, 0x00, 0x7a, 0xdf, 0xc6, 0x52, 0xf2, 0xcc, 0x6d, 0x21, 0x6c, 0x2c, 0x42, 0xa0,
	0xb3, 0x4c, 0xf2, 0xdc, 0x6d, 0x23, 0x8a, 0xdf, 0xe4, 0x19, 0x1c, 0xbc, 0x62, 0x22, 0xfc, 0x21,
	0x0e, 0x65, 0xe4, 0x76, 0xce, 0x9c, 0x49, 0x87, 0x96, 0x00, 0xf9, 0x18, 0x46, 0x57, 0x19, 0xe7,
	0x25, 0xa3, 0x8b, 0x8c, 0x2a, 0xe8, 0xfd, 0xe1, 0x00, 0x5c, 0xc5, 0x7c, 0x1d, 0x62, 0x5c, 0xe4,
	0x0c, 0x86, 0xcb, 0x24, 0x60, 0xeb, 0x55, 0x12, 0x72, 0x7f, 0x85, 0xa1, 0x1d, 0x50, 0x1b, 0x22,
	0x1e, 0x1c, 0x52, 0xfe, 0x98, 0x48, 0x6e, 0x28, 0x2d, 0xa4, 0x54, 0x30, 0x72, 0x0e, 0x63, 0x5c,
	0xb2, 0x10, 0x92, 0x67, 0xf7, 0x2c, 0xe0, 0x18, 0xf6, 0x01, 0xad, 0xa1, 0xe4, 0x29, 0x0c, 0x66,
	0x52, 0xb2, 0x20, 0x5a, 0x84, 0x26, 0xfe, 0x9d, 0x4d, 0x5e, 0xc0, 0x61, 0x21, 0xd5, 0x4c, 0xca,
	0x0c, 0xa3, 0x1f, 0x4e, 0x8f, 0x2e, 0x50, 0xc9, 0x8b, 0xc2, 0x45, 0x2b, 0x24, 0x15, 0xfe, 0x77,
	0x29, 0x7b, 0xb7, 0xe1, 0xb7, 0x6c, 0xbd, 0xe1, 0x6e, 0x4f, 0x87, 0x6f, 0x41, 0xde, 0x2f, 0x1d,
	0x18, 0xce, 0x93, 0x47, 0x16, 0x0b, 0x9d, 0xf0, 0x04, 0x8e, 0x30, 0x28, 0x8d, 0xad, 0xd8, 0x23,
	0x37, 0x49, 0xd7, 0x61, 0xa5, 0xa7, 0x05, 0x2d, 0x42, 0x53, 0xa0, 0x2a, 0x48, 0x3e, 0x81, 0x63,
	0x2d, 0x85, 0xb5, 0xa1, 0x4e, 0xbe, 0x81, 0x2b, 0x99, 0x6c, 0xcc, 0x88, 0x30, 0xa2, 0x35, 0xb4,
	0x5e, 0x94, 0xee, 0xbf, 0x17, 0xa5, 0xf7, 0x5e, 0x45, 0xe9, 0xef, 0x2d, 0xca, 0x39, 0x8c, 0x75,
	0x11, 0x76, 0x51, 0x0d, 0xb0, 0x34, 0x35, 0xb4, 0x51, 0xa0, 0x83, 0xff, 0x50, 0x20, 0x68, 0x14,
	0x48, 0x09, 0x68, 0x1f, 0x84, 0x02, 0x0e, 0xb5, 0x80, 0x75, 0x5c, 0x95, 0x64, 0x21, 0x1e, 0x32,
	0x9e, 0xe7, 0x6f, 0x38, 0xcf, 0xfc, 0x95, 0x7b, 0x88, 0xc4, 0x2a, 0xa8, 0xc4, 0xf9, 0xc6, 0x26,
	0x8d, 0xb4, 0x38, 0x36, 0x46, 0x8e, 0xa1, 0xbd, 0xc8, 0x53, 0x77, 0x8c, 0x2e, 0xf5, 0xe9, 0xfd,
	0xe6, 0xc0, 0x29, 0x5e, 0x8c, 0x9b, 0x24, 0x4d, 0x2e, 0x59, 0x10, 0xf1, 0x55, 0x22, 0xe3, 0xfb,
	0xad, 0x4a, 0xc1, 0xe7, 0xef, 0x36, 0x5c, 0x04, 0x7c, 0xb5, 0x79, 0xc4, 0x6e, 0xe9, 0x50, 0x1b,
	0x7a, 0xcf, 0x4e, 0xd9, 0xd3, 0x79, 0xed, 0xfd, 0x9d, 0x57, 0xab, 0x7f, 0xa7, 0x59, 0xff, 0x2f,
	0xe1, 0xa8, 0xbc, 0xc4, 0xb3, 0x2c, 0x63, 0x5b, 0xb7, 0x7b, 0xd6, 0x9e, 0x0c, 0xa7, 0x27, 0xa6,
	0x1c, 0xa5, 0x97, 0xd6, 0x99, 0xde, 0xef, 0x0e, 0x3c, 0xd1, 0xa7, 0xfd, 0x1f, 0x52, 0xfd, 0x1a,
	0x8e, 0xad, 0xfb, 0x6b, 0xe7, 0x4a, 0x4c, 0xae, 0x96, 0x9b, 0x36, 0xb8, 0xde, 0xe7, 0xd0, 0x5f,
	0x84, 0x53, 0x3c, 0xec, 0x29, 0x0c, 0x4c, 0x88, 0x73, 0x73, 0xe9, 0x77, 0xb6, 0x7a, 0x6f, 0x31,
	0x4e, 0xfd, 0xbc, 0xe1, 0xb7, 0xf7, 0x16, 0x4e, 0x94, 0x42, 0x6f, 0x63, 0x19, 0x5d, 0xb1, 0xbb,
	0x2c, 0x0e, 0x5e, 0xb3, 0x94, 0x4c, 0xa0, 0xaf, 0x9c, 0xaf, 0x59, 0xea, 0x3a, 0x18, 0xc6, 0xd8,
	0x84, 0x61, 0x4e, 0xa1, 0x85, 0x9b, 0xb8, 0xd0, 0xbf, 0x4c, 0x84, 0xe4, 0x42, 0xab, 0x74, 0x48,
	0x0b, 0xd3, 0x4b, 0xe1, 0xc3, 0x57, 0xb1, 0x08, 0x63, 0xf1, 0xe0, 0xf3, 0x35, 0x0f, 0x24, 0x0f,
	0x75, 0x1c, 0x6f, 0x98, 0x8c, 0x88, 0x0f, 0xa4, 0x89, 0x9a, 0xb3, 0x3e, 0x32, 0x67, 0xcd, 0xd2,
	0xf4, 0x32, 0x11, 0x82, 0x07, 0xb2, 0x49, 0xa5, 0x7b, 0x96, 0x7b, 0x3f, 0x3b, 0xf0, 0xec, 0x9f,
	0x16, 0x91, 0x97, 0x00, 0xa5, 0x1f, 0xd5, 0x19, 0x4e, 0x9f, 0x34, 0x4e, 0x53, 0x77, 0x9a, 0x5a,
	0x44, 0xf2, 0x19, 0x80, 0xde, 0x64, 0x19, 0xe7, 0xd2, 0x6d, 0x55, 0x7a, 0xd0, 0x68, 0x2b, 0xee,
	0x13, 0x6a, 0x91, 0x4a, 0x59, 0x24, 0x36, 0xc5, 0x4e, 0x16, 0xe9, 0xbd, 0x84, 0xe1, 0xf5, 0xad,
	0x3a, 0x22, 0xbe, 0xdb, 0x48, 0xae, 0xee, 0xe8, 0x35, 0xdf, 0x9a, 0x4a, 0xa9, 0x4f, 0xf5, 0x0b,
	0xd5, 0xef, 0x88, 0xae, 0x92, 0x36, 0xbc, 0x3f, 0x1d, 0x18, 0x57, 0x43, 0x24, 0xe7, 0xe5, 0xd2,
	0xe1, 0xf4, 0xb4, 0x91, 0xc6, 0x35, 0xdf, 0xea, 0x0d, 0x3f, 0x85, 0x7e, 0xf1, 0x9c, 0xb5, 0x90,
	0x7b, 0x52, 0x72, 0x8d, 0x83, 0x16, 0x0c, 0xf2, 0x05, 0x8c, 0xfd, 0x2c, 0xf0, 0x03, 0x11, 0x87,
	0xd7, 0xb7, 0x98, 0x6f, 0xbb, 0xd2, 0x87, 0x56, 0xec, 0xb4, 0xc6, 0x24, 0x5f, 0xc1, 0xd1, 0x9c,
	0xe7, 0xd2, 0x5e, 0xdc, 0xf9, 0xdb, 0xc5, 0x75, 0x2a, 0x79, 0x0e, 0x30, 0x0b, 0x02, 0xbe, 0xe6,
	0x19, 0x93, 0x1c, 0xff, 0x07, 0x03, 0x6a, 0x21, 0xde, 0xaf, 0x0e, 0x8c, 0x2a, 0xd9, 0xa9, 0x56,
	0x57, 0x11, 0x5c, 0xae, 0xca, 0x56, 0x2f, 0x6c, 0xbc, 0x06, 0xb9, 0xd4, 0xbe, 0x96, 0xb9, 0x06,
	0xc6, 0x56, 0x0a, 0xfb, 0x59, 0xb0, 0x98, 0x9b, 0xb9, 0x43, 0x1b, 0x38, 0xba, 0xe4, 0x72, 0x31,
	0x37, 0xff, 0x2b, 0x6d, 0xe0, 0x6b, 0x91, 0x05, 0xbb, 0x97, 0xa0, 0x8b, 0x3e, 0x1b, 0x52, 0x8c,
	0x79, 0x2e, 0x77, 0x8c, 0x9e, 0x66, 0x58, 0x90, 0xf7, 0x93, 0x83, 0x7d, 0x57, 0x48, 0xfc, 0x1c,
	0x00, 0xc7, 0x22, 0x5d, 0x65, 0x3d, 0x28, 0x59, 0x88, 0x9a, 0x80, 0x96, 0x49, 0x2e, 0xcb, 0x26,
	0x18, 0xd1, 0x12, 0x50, 0xc7, 0xe9, 0xe9, 0x49, 0xfb, 0x75, 0x0a, 0x36, 0xa4, 0x1e, 0xa6, 0x9b,
	0x28, 0x4b, 0x36, 0x0f, 0x51, 0xba, 0x31, 0xbb, 0xe8, 0x39, 0xa4, 0x0e, 0x7b, 0x51, 0xd1, 0xd8,
	0xaa, 0x7f, 0x31, 0xae, 0xfa, 0xc0, 0x60, 0x21, 0xd6, 0xcb, 0x52, 0xbc, 0x88, 0x3b, 0xbb, 0x5c,
	0x7b, 0xb3, 0x4d, 0x8b, 0xa0, 0x2c, 0xe4, 0xae, 0x87, 0x23, 0xe3, 0x8b, 0xbf, 0x02, 0x00, 0x00,
	0xff, 0xff, 0x59, 0x30, 0x2e, 0x35, 0x3e, 0x0a, 0x00, 0x00,
}

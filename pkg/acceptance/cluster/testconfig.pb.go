// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: acceptance/cluster/testconfig.proto

/*
	Package cluster is a generated protocol buffer package.

	It is generated from these files:
		acceptance/cluster/testconfig.proto

	It has these top-level messages:
		StoreConfig
		NodeConfig
		TestConfig
*/
package cluster

import proto "github.com/gogo/protobuf/proto"
import fmt "fmt"
import math "math"

import time "time"

import io "io"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf
var _ = time.Kitchen

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion2 // please upgrade the proto package

// InitMode specifies different ways to initialize the cluster.
type InitMode int32

const (
	// INIT_COMMAND starts every node with a join flag and issues the
	// init command.
	INIT_COMMAND InitMode = 0
	// INIT_BOOTSTRAP_NODE_ZERO uses the legacy protocol of omitting the
	// join flag from node zero.
	INIT_BOOTSTRAP_NODE_ZERO InitMode = 1
	// INIT_NONE starts every node with a join flag and leaves the
	// cluster uninitialized.
	INIT_NONE InitMode = 2
)

var InitMode_name = map[int32]string{
	0: "INIT_COMMAND",
	1: "INIT_BOOTSTRAP_NODE_ZERO",
	2: "INIT_NONE",
}
var InitMode_value = map[string]int32{
	"INIT_COMMAND":             0,
	"INIT_BOOTSTRAP_NODE_ZERO": 1,
	"INIT_NONE":                2,
}

func (x InitMode) Enum() *InitMode {
	p := new(InitMode)
	*p = x
	return p
}
func (x InitMode) String() string {
	return proto.EnumName(InitMode_name, int32(x))
}
func (x *InitMode) UnmarshalJSON(data []byte) error {
	value, err := proto.UnmarshalJSONEnum(InitMode_value, data, "InitMode")
	if err != nil {
		return err
	}
	*x = InitMode(value)
	return nil
}
func (InitMode) EnumDescriptor() ([]byte, []int) { return fileDescriptorTestconfig, []int{0} }

// StoreConfig holds the configuration of a collection of similar stores.
type StoreConfig struct {
	MaxRanges int32 `protobuf:"varint,2,opt,name=max_ranges,json=maxRanges" json:"max_ranges"`
}

func (m *StoreConfig) Reset()                    { *m = StoreConfig{} }
func (m *StoreConfig) String() string            { return proto.CompactTextString(m) }
func (*StoreConfig) ProtoMessage()               {}
func (*StoreConfig) Descriptor() ([]byte, []int) { return fileDescriptorTestconfig, []int{0} }

// NodeConfig holds the configuration of a collection of similar nodes.
type NodeConfig struct {
	Version string        `protobuf:"bytes,1,opt,name=version" json:"version"`
	Stores  []StoreConfig `protobuf:"bytes,2,rep,name=stores" json:"stores"`
}

func (m *NodeConfig) Reset()                    { *m = NodeConfig{} }
func (m *NodeConfig) String() string            { return proto.CompactTextString(m) }
func (*NodeConfig) ProtoMessage()               {}
func (*NodeConfig) Descriptor() ([]byte, []int) { return fileDescriptorTestconfig, []int{1} }

type TestConfig struct {
	Name  string       `protobuf:"bytes,1,opt,name=name" json:"name"`
	Nodes []NodeConfig `protobuf:"bytes,2,rep,name=nodes" json:"nodes"`
	// Duration is the total time that the test should run for. Important for
	// tests such as TestPut that will run indefinitely.
	Duration time.Duration `protobuf:"varint,3,opt,name=duration,casttype=time.Duration" json:"duration"`
	InitMode InitMode      `protobuf:"varint,4,opt,name=init_mode,json=initMode,enum=cockroach.acceptance.cluster.InitMode" json:"init_mode"`
	// When set, the cluster is started as quickly as possible, without waiting
	// for ranges to replicate, or even ports to be opened.
	NoWait bool `protobuf:"varint,5,opt,name=no_wait,json=noWait" json:"no_wait"`
}

func (m *TestConfig) Reset()                    { *m = TestConfig{} }
func (m *TestConfig) String() string            { return proto.CompactTextString(m) }
func (*TestConfig) ProtoMessage()               {}
func (*TestConfig) Descriptor() ([]byte, []int) { return fileDescriptorTestconfig, []int{2} }

func init() {
	proto.RegisterType((*StoreConfig)(nil), "cockroach.acceptance.cluster.StoreConfig")
	proto.RegisterType((*NodeConfig)(nil), "cockroach.acceptance.cluster.NodeConfig")
	proto.RegisterType((*TestConfig)(nil), "cockroach.acceptance.cluster.TestConfig")
	proto.RegisterEnum("cockroach.acceptance.cluster.InitMode", InitMode_name, InitMode_value)
}
func (m *StoreConfig) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *StoreConfig) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	dAtA[i] = 0x10
	i++
	i = encodeVarintTestconfig(dAtA, i, uint64(m.MaxRanges))
	return i, nil
}

func (m *NodeConfig) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *NodeConfig) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	dAtA[i] = 0xa
	i++
	i = encodeVarintTestconfig(dAtA, i, uint64(len(m.Version)))
	i += copy(dAtA[i:], m.Version)
	if len(m.Stores) > 0 {
		for _, msg := range m.Stores {
			dAtA[i] = 0x12
			i++
			i = encodeVarintTestconfig(dAtA, i, uint64(msg.Size()))
			n, err := msg.MarshalTo(dAtA[i:])
			if err != nil {
				return 0, err
			}
			i += n
		}
	}
	return i, nil
}

func (m *TestConfig) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *TestConfig) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	dAtA[i] = 0xa
	i++
	i = encodeVarintTestconfig(dAtA, i, uint64(len(m.Name)))
	i += copy(dAtA[i:], m.Name)
	if len(m.Nodes) > 0 {
		for _, msg := range m.Nodes {
			dAtA[i] = 0x12
			i++
			i = encodeVarintTestconfig(dAtA, i, uint64(msg.Size()))
			n, err := msg.MarshalTo(dAtA[i:])
			if err != nil {
				return 0, err
			}
			i += n
		}
	}
	dAtA[i] = 0x18
	i++
	i = encodeVarintTestconfig(dAtA, i, uint64(m.Duration))
	dAtA[i] = 0x20
	i++
	i = encodeVarintTestconfig(dAtA, i, uint64(m.InitMode))
	dAtA[i] = 0x28
	i++
	if m.NoWait {
		dAtA[i] = 1
	} else {
		dAtA[i] = 0
	}
	i++
	return i, nil
}

func encodeVarintTestconfig(dAtA []byte, offset int, v uint64) int {
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return offset + 1
}
func (m *StoreConfig) Size() (n int) {
	var l int
	_ = l
	n += 1 + sovTestconfig(uint64(m.MaxRanges))
	return n
}

func (m *NodeConfig) Size() (n int) {
	var l int
	_ = l
	l = len(m.Version)
	n += 1 + l + sovTestconfig(uint64(l))
	if len(m.Stores) > 0 {
		for _, e := range m.Stores {
			l = e.Size()
			n += 1 + l + sovTestconfig(uint64(l))
		}
	}
	return n
}

func (m *TestConfig) Size() (n int) {
	var l int
	_ = l
	l = len(m.Name)
	n += 1 + l + sovTestconfig(uint64(l))
	if len(m.Nodes) > 0 {
		for _, e := range m.Nodes {
			l = e.Size()
			n += 1 + l + sovTestconfig(uint64(l))
		}
	}
	n += 1 + sovTestconfig(uint64(m.Duration))
	n += 1 + sovTestconfig(uint64(m.InitMode))
	n += 2
	return n
}

func sovTestconfig(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
func sozTestconfig(x uint64) (n int) {
	return sovTestconfig(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *StoreConfig) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowTestconfig
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: StoreConfig: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: StoreConfig: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field MaxRanges", wireType)
			}
			m.MaxRanges = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTestconfig
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.MaxRanges |= (int32(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipTestconfig(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthTestconfig
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *NodeConfig) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowTestconfig
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: NodeConfig: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: NodeConfig: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Version", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTestconfig
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthTestconfig
			}
			postIndex := iNdEx + intStringLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Version = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Stores", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTestconfig
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthTestconfig
			}
			postIndex := iNdEx + msglen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Stores = append(m.Stores, StoreConfig{})
			if err := m.Stores[len(m.Stores)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipTestconfig(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthTestconfig
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *TestConfig) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowTestconfig
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: TestConfig: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: TestConfig: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Name", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTestconfig
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthTestconfig
			}
			postIndex := iNdEx + intStringLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Name = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Nodes", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTestconfig
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthTestconfig
			}
			postIndex := iNdEx + msglen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Nodes = append(m.Nodes, NodeConfig{})
			if err := m.Nodes[len(m.Nodes)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Duration", wireType)
			}
			m.Duration = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTestconfig
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Duration |= (time.Duration(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 4:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field InitMode", wireType)
			}
			m.InitMode = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTestconfig
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.InitMode |= (InitMode(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 5:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field NoWait", wireType)
			}
			var v int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTestconfig
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			m.NoWait = bool(v != 0)
		default:
			iNdEx = preIndex
			skippy, err := skipTestconfig(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthTestconfig
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipTestconfig(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowTestconfig
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowTestconfig
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
			return iNdEx, nil
		case 1:
			iNdEx += 8
			return iNdEx, nil
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowTestconfig
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			iNdEx += length
			if length < 0 {
				return 0, ErrInvalidLengthTestconfig
			}
			return iNdEx, nil
		case 3:
			for {
				var innerWire uint64
				var start int = iNdEx
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return 0, ErrIntOverflowTestconfig
					}
					if iNdEx >= l {
						return 0, io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					innerWire |= (uint64(b) & 0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				innerWireType := int(innerWire & 0x7)
				if innerWireType == 4 {
					break
				}
				next, err := skipTestconfig(dAtA[start:])
				if err != nil {
					return 0, err
				}
				iNdEx = start + next
			}
			return iNdEx, nil
		case 4:
			return iNdEx, nil
		case 5:
			iNdEx += 4
			return iNdEx, nil
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
	}
	panic("unreachable")
}

var (
	ErrInvalidLengthTestconfig = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowTestconfig   = fmt.Errorf("proto: integer overflow")
)

func init() { proto.RegisterFile("acceptance/cluster/testconfig.proto", fileDescriptorTestconfig) }

var fileDescriptorTestconfig = []byte{
	// 415 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x7c, 0x90, 0xcd, 0x6e, 0xd3, 0x40,
	0x14, 0x85, 0x3d, 0xf9, 0x21, 0xc9, 0x2d, 0x45, 0xd1, 0x08, 0x24, 0xab, 0x2a, 0x53, 0x93, 0x4a,
	0xc8, 0xb0, 0x70, 0x44, 0xde, 0xa0, 0xa9, 0x23, 0xe4, 0x45, 0x6c, 0xe4, 0x5a, 0x42, 0xea, 0xc6,
	0x1a, 0x8d, 0x07, 0x33, 0x02, 0xcf, 0x54, 0xf6, 0x04, 0xfa, 0x08, 0xb0, 0xe3, 0x1d, 0x78, 0x99,
	0x2c, 0x59, 0xb2, 0xaa, 0xc0, 0xbc, 0x05, 0x2b, 0x64, 0x77, 0xdc, 0xd0, 0x4d, 0x76, 0xf6, 0xb9,
	0xe7, 0x9c, 0xef, 0xde, 0x81, 0x53, 0xca, 0x18, 0xbf, 0xd2, 0x54, 0x32, 0x3e, 0x67, 0x1f, 0x37,
	0x95, 0xe6, 0xe5, 0x5c, 0xf3, 0x4a, 0x33, 0x25, 0xdf, 0x89, 0xdc, 0xbb, 0x2a, 0x95, 0x56, 0xf8,
	0x98, 0x29, 0xf6, 0xa1, 0x54, 0x94, 0xbd, 0xf7, 0x76, 0x76, 0xcf, 0xd8, 0x8f, 0x1e, 0xe7, 0x2a,
	0x57, 0xad, 0x71, 0xde, 0x7c, 0xdd, 0x66, 0x66, 0x0b, 0x38, 0xb8, 0xd0, 0xaa, 0xe4, 0xe7, 0x6d,
	0x11, 0x3e, 0x05, 0x28, 0xe8, 0x75, 0x5a, 0x52, 0x99, 0xf3, 0xca, 0xee, 0x39, 0xc8, 0x1d, 0x2e,
	0x07, 0xdb, 0x9b, 0x13, 0x2b, 0x9e, 0x14, 0xf4, 0x3a, 0x6e, 0xe5, 0xd9, 0x06, 0x20, 0x54, 0x59,
	0x17, 0x21, 0x30, 0xfa, 0xc4, 0xcb, 0x4a, 0x28, 0x69, 0x23, 0x07, 0xb9, 0x13, 0xe3, 0xef, 0x44,
	0xfc, 0x1a, 0x1e, 0x54, 0x0d, 0xa1, 0xa9, 0xeb, 0xbb, 0x07, 0x8b, 0x17, 0xde, 0xbe, 0x35, 0xbd,
	0xff, 0xb6, 0x31, 0x4d, 0x26, 0x3e, 0xfb, 0xda, 0x03, 0x48, 0x78, 0xa5, 0x0d, 0xd7, 0x86, 0x81,
	0xa4, 0x05, 0xbf, 0x07, 0x6d, 0x15, 0xec, 0xc3, 0x50, 0xaa, 0xec, 0x0e, 0xe8, 0xee, 0x07, 0xee,
	0x4e, 0x31, 0x25, 0xb7, 0x61, 0xfc, 0x0a, 0xc6, 0xd9, 0xa6, 0xa4, 0xba, 0x39, 0xac, 0xef, 0x20,
	0xb7, 0xbf, 0x7c, 0xd2, 0x8c, 0xff, 0xde, 0x9c, 0x1c, 0x6a, 0x51, 0x70, 0xcf, 0x37, 0xc3, 0xf8,
	0xce, 0x86, 0x03, 0x98, 0x08, 0x29, 0x74, 0x5a, 0xa8, 0x8c, 0xdb, 0x03, 0x07, 0xb9, 0x8f, 0x16,
	0xcf, 0xf7, 0xc3, 0x03, 0x29, 0xf4, 0x5a, 0x65, 0xdc, 0xa0, 0xc7, 0xc2, 0xfc, 0xe3, 0xa7, 0x30,
	0x92, 0x2a, 0xfd, 0x4c, 0x85, 0xb6, 0x87, 0x0e, 0x72, 0xc7, 0xdd, 0x5b, 0x48, 0xf5, 0x96, 0x0a,
	0xfd, 0x32, 0x82, 0x71, 0x17, 0xc5, 0x53, 0x78, 0x18, 0x84, 0x41, 0x92, 0x9e, 0x47, 0xeb, 0xf5,
	0x59, 0xe8, 0x4f, 0x2d, 0x7c, 0x0c, 0x76, 0xab, 0x2c, 0xa3, 0x28, 0xb9, 0x48, 0xe2, 0xb3, 0x37,
	0x69, 0x18, 0xf9, 0xab, 0xf4, 0x72, 0x15, 0x47, 0x53, 0x84, 0x0f, 0x61, 0xd2, 0x4e, 0xc3, 0x28,
	0x5c, 0x4d, 0x7b, 0x47, 0x83, 0x2f, 0xdf, 0x89, 0xb5, 0x7c, 0xb6, 0xfd, 0x4d, 0xac, 0x6d, 0x4d,
	0xd0, 0x8f, 0x9a, 0xa0, 0x9f, 0x35, 0x41, 0xbf, 0x6a, 0x82, 0xbe, 0xfd, 0x21, 0xd6, 0xe5, 0xc8,
	0xec, 0xfa, 0x2f, 0x00, 0x00, 0xff, 0xff, 0xa9, 0x6a, 0x84, 0xa9, 0x84, 0x02, 0x00, 0x00,
}

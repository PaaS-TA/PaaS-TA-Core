// Code generated by protoc-gen-gogo.
// source: volume_mount.proto
// DO NOT EDIT!

package models

import proto "github.com/gogo/protobuf/proto"
import math "math"

// discarding unused import gogoproto "github.com/gogo/protobuf/gogoproto"

import strconv "strconv"

import bytes "bytes"

import fmt "fmt"
import strings "strings"
import github_com_gogo_protobuf_proto "github.com/gogo/protobuf/proto"
import sort "sort"
import reflect "reflect"

import io "io"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = math.Inf

type BindMountMode int32

const (
	BindMountMode_RO BindMountMode = 0
	BindMountMode_RW BindMountMode = 1
)

var BindMountMode_name = map[int32]string{
	0: "RO",
	1: "RW",
}
var BindMountMode_value = map[string]int32{
	"RO": 0,
	"RW": 1,
}

func (x BindMountMode) Enum() *BindMountMode {
	p := new(BindMountMode)
	*p = x
	return p
}
func (x BindMountMode) MarshalJSON() ([]byte, error) {
	return proto.MarshalJSONEnum(BindMountMode_name, int32(x))
}
func (x *BindMountMode) UnmarshalJSON(data []byte) error {
	value, err := proto.UnmarshalJSONEnum(BindMountMode_value, data, "BindMountMode")
	if err != nil {
		return err
	}
	*x = BindMountMode(value)
	return nil
}

type VolumeMount struct {
	Driver        string        `protobuf:"bytes,1,opt,name=driver" json:"driver"`
	VolumeId      string        `protobuf:"bytes,2,opt,name=volume_id" json:"volume_id"`
	ContainerPath string        `protobuf:"bytes,3,opt,name=container_path" json:"container_path"`
	Mode          BindMountMode `protobuf:"varint,4,opt,name=mode,enum=models.BindMountMode" json:"mode"`
	Config        []byte        `protobuf:"bytes,5,opt,name=config" json:"config,omitempty"`
}

func (m *VolumeMount) Reset()      { *m = VolumeMount{} }
func (*VolumeMount) ProtoMessage() {}

func (m *VolumeMount) GetDriver() string {
	if m != nil {
		return m.Driver
	}
	return ""
}

func (m *VolumeMount) GetVolumeId() string {
	if m != nil {
		return m.VolumeId
	}
	return ""
}

func (m *VolumeMount) GetContainerPath() string {
	if m != nil {
		return m.ContainerPath
	}
	return ""
}

func (m *VolumeMount) GetMode() BindMountMode {
	if m != nil {
		return m.Mode
	}
	return BindMountMode_RO
}

func (m *VolumeMount) GetConfig() []byte {
	if m != nil {
		return m.Config
	}
	return nil
}

type VolumePlacement struct {
	DriverNames []string `protobuf:"bytes,1,rep,name=driver_names" json:"driver_names"`
}

func (m *VolumePlacement) Reset()      { *m = VolumePlacement{} }
func (*VolumePlacement) ProtoMessage() {}

func (m *VolumePlacement) GetDriverNames() []string {
	if m != nil {
		return m.DriverNames
	}
	return nil
}

func init() {
	proto.RegisterEnum("models.BindMountMode", BindMountMode_name, BindMountMode_value)
}
func (x BindMountMode) String() string {
	s, ok := BindMountMode_name[int32(x)]
	if ok {
		return s
	}
	return strconv.Itoa(int(x))
}
func (this *VolumeMount) Equal(that interface{}) bool {
	if that == nil {
		if this == nil {
			return true
		}
		return false
	}

	that1, ok := that.(*VolumeMount)
	if !ok {
		return false
	}
	if that1 == nil {
		if this == nil {
			return true
		}
		return false
	} else if this == nil {
		return false
	}
	if this.Driver != that1.Driver {
		return false
	}
	if this.VolumeId != that1.VolumeId {
		return false
	}
	if this.ContainerPath != that1.ContainerPath {
		return false
	}
	if this.Mode != that1.Mode {
		return false
	}
	if !bytes.Equal(this.Config, that1.Config) {
		return false
	}
	return true
}
func (this *VolumePlacement) Equal(that interface{}) bool {
	if that == nil {
		if this == nil {
			return true
		}
		return false
	}

	that1, ok := that.(*VolumePlacement)
	if !ok {
		return false
	}
	if that1 == nil {
		if this == nil {
			return true
		}
		return false
	} else if this == nil {
		return false
	}
	if len(this.DriverNames) != len(that1.DriverNames) {
		return false
	}
	for i := range this.DriverNames {
		if this.DriverNames[i] != that1.DriverNames[i] {
			return false
		}
	}
	return true
}
func (this *VolumeMount) GoString() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&models.VolumeMount{` +
		`Driver:` + fmt.Sprintf("%#v", this.Driver),
		`VolumeId:` + fmt.Sprintf("%#v", this.VolumeId),
		`ContainerPath:` + fmt.Sprintf("%#v", this.ContainerPath),
		`Mode:` + fmt.Sprintf("%#v", this.Mode),
		`Config:` + valueToGoStringVolumeMount(this.Config, "byte") + `}`}, ", ")
	return s
}
func (this *VolumePlacement) GoString() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&models.VolumePlacement{` +
		`DriverNames:` + fmt.Sprintf("%#v", this.DriverNames) + `}`}, ", ")
	return s
}
func valueToGoStringVolumeMount(v interface{}, typ string) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("func(v %v) *%v { return &v } ( %#v )", typ, typ, pv)
}
func extensionToGoStringVolumeMount(e map[int32]github_com_gogo_protobuf_proto.Extension) string {
	if e == nil {
		return "nil"
	}
	s := "map[int32]proto.Extension{"
	keys := make([]int, 0, len(e))
	for k := range e {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	ss := []string{}
	for _, k := range keys {
		ss = append(ss, strconv.Itoa(k)+": "+e[int32(k)].GoString())
	}
	s += strings.Join(ss, ",") + "}"
	return s
}
func (m *VolumeMount) Marshal() (data []byte, err error) {
	size := m.Size()
	data = make([]byte, size)
	n, err := m.MarshalTo(data)
	if err != nil {
		return nil, err
	}
	return data[:n], nil
}

func (m *VolumeMount) MarshalTo(data []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	data[i] = 0xa
	i++
	i = encodeVarintVolumeMount(data, i, uint64(len(m.Driver)))
	i += copy(data[i:], m.Driver)
	data[i] = 0x12
	i++
	i = encodeVarintVolumeMount(data, i, uint64(len(m.VolumeId)))
	i += copy(data[i:], m.VolumeId)
	data[i] = 0x1a
	i++
	i = encodeVarintVolumeMount(data, i, uint64(len(m.ContainerPath)))
	i += copy(data[i:], m.ContainerPath)
	data[i] = 0x20
	i++
	i = encodeVarintVolumeMount(data, i, uint64(m.Mode))
	if m.Config != nil {
		data[i] = 0x2a
		i++
		i = encodeVarintVolumeMount(data, i, uint64(len(m.Config)))
		i += copy(data[i:], m.Config)
	}
	return i, nil
}

func (m *VolumePlacement) Marshal() (data []byte, err error) {
	size := m.Size()
	data = make([]byte, size)
	n, err := m.MarshalTo(data)
	if err != nil {
		return nil, err
	}
	return data[:n], nil
}

func (m *VolumePlacement) MarshalTo(data []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if len(m.DriverNames) > 0 {
		for _, s := range m.DriverNames {
			data[i] = 0xa
			i++
			l = len(s)
			for l >= 1<<7 {
				data[i] = uint8(uint64(l)&0x7f | 0x80)
				l >>= 7
				i++
			}
			data[i] = uint8(l)
			i++
			i += copy(data[i:], s)
		}
	}
	return i, nil
}

func encodeFixed64VolumeMount(data []byte, offset int, v uint64) int {
	data[offset] = uint8(v)
	data[offset+1] = uint8(v >> 8)
	data[offset+2] = uint8(v >> 16)
	data[offset+3] = uint8(v >> 24)
	data[offset+4] = uint8(v >> 32)
	data[offset+5] = uint8(v >> 40)
	data[offset+6] = uint8(v >> 48)
	data[offset+7] = uint8(v >> 56)
	return offset + 8
}
func encodeFixed32VolumeMount(data []byte, offset int, v uint32) int {
	data[offset] = uint8(v)
	data[offset+1] = uint8(v >> 8)
	data[offset+2] = uint8(v >> 16)
	data[offset+3] = uint8(v >> 24)
	return offset + 4
}
func encodeVarintVolumeMount(data []byte, offset int, v uint64) int {
	for v >= 1<<7 {
		data[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	data[offset] = uint8(v)
	return offset + 1
}
func (m *VolumeMount) Size() (n int) {
	var l int
	_ = l
	l = len(m.Driver)
	n += 1 + l + sovVolumeMount(uint64(l))
	l = len(m.VolumeId)
	n += 1 + l + sovVolumeMount(uint64(l))
	l = len(m.ContainerPath)
	n += 1 + l + sovVolumeMount(uint64(l))
	n += 1 + sovVolumeMount(uint64(m.Mode))
	if m.Config != nil {
		l = len(m.Config)
		n += 1 + l + sovVolumeMount(uint64(l))
	}
	return n
}

func (m *VolumePlacement) Size() (n int) {
	var l int
	_ = l
	if len(m.DriverNames) > 0 {
		for _, s := range m.DriverNames {
			l = len(s)
			n += 1 + l + sovVolumeMount(uint64(l))
		}
	}
	return n
}

func sovVolumeMount(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
func sozVolumeMount(x uint64) (n int) {
	return sovVolumeMount(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (this *VolumeMount) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&VolumeMount{`,
		`Driver:` + fmt.Sprintf("%v", this.Driver) + `,`,
		`VolumeId:` + fmt.Sprintf("%v", this.VolumeId) + `,`,
		`ContainerPath:` + fmt.Sprintf("%v", this.ContainerPath) + `,`,
		`Mode:` + fmt.Sprintf("%v", this.Mode) + `,`,
		`Config:` + valueToStringVolumeMount(this.Config) + `,`,
		`}`,
	}, "")
	return s
}
func (this *VolumePlacement) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&VolumePlacement{`,
		`DriverNames:` + fmt.Sprintf("%v", this.DriverNames) + `,`,
		`}`,
	}, "")
	return s
}
func valueToStringVolumeMount(v interface{}) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("*%v", pv)
}
func (m *VolumeMount) Unmarshal(data []byte) error {
	l := len(data)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := data[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Driver", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				stringLen |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			postIndex := iNdEx + int(stringLen)
			if stringLen < 0 {
				return ErrInvalidLengthVolumeMount
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Driver = string(data[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field VolumeId", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				stringLen |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			postIndex := iNdEx + int(stringLen)
			if stringLen < 0 {
				return ErrInvalidLengthVolumeMount
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.VolumeId = string(data[iNdEx:postIndex])
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ContainerPath", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				stringLen |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			postIndex := iNdEx + int(stringLen)
			if stringLen < 0 {
				return ErrInvalidLengthVolumeMount
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.ContainerPath = string(data[iNdEx:postIndex])
			iNdEx = postIndex
		case 4:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Mode", wireType)
			}
			m.Mode = 0
			for shift := uint(0); ; shift += 7 {
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				m.Mode |= (BindMountMode(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 5:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Config", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				byteLen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthVolumeMount
			}
			postIndex := iNdEx + byteLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Config = append([]byte{}, data[iNdEx:postIndex]...)
			iNdEx = postIndex
		default:
			var sizeOfWire int
			for {
				sizeOfWire++
				wire >>= 7
				if wire == 0 {
					break
				}
			}
			iNdEx -= sizeOfWire
			skippy, err := skipVolumeMount(data[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthVolumeMount
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	return nil
}
func (m *VolumePlacement) Unmarshal(data []byte) error {
	l := len(data)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := data[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field DriverNames", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				stringLen |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			postIndex := iNdEx + int(stringLen)
			if stringLen < 0 {
				return ErrInvalidLengthVolumeMount
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.DriverNames = append(m.DriverNames, string(data[iNdEx:postIndex]))
			iNdEx = postIndex
		default:
			var sizeOfWire int
			for {
				sizeOfWire++
				wire >>= 7
				if wire == 0 {
					break
				}
			}
			iNdEx -= sizeOfWire
			skippy, err := skipVolumeMount(data[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthVolumeMount
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	return nil
}
func skipVolumeMount(data []byte) (n int, err error) {
	l := len(data)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := data[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for {
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if data[iNdEx-1] < 0x80 {
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
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			iNdEx += length
			if length < 0 {
				return 0, ErrInvalidLengthVolumeMount
			}
			return iNdEx, nil
		case 3:
			for {
				var innerWire uint64
				var start int = iNdEx
				for shift := uint(0); ; shift += 7 {
					if iNdEx >= l {
						return 0, io.ErrUnexpectedEOF
					}
					b := data[iNdEx]
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
				next, err := skipVolumeMount(data[start:])
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
	ErrInvalidLengthVolumeMount = fmt.Errorf("proto: negative length found during unmarshaling")
)
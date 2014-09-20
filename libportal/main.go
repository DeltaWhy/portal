package libportal

type PacketKind uint16

const (
	Ping PacketKind = 0
	AuthReq = 1
	AuthResp = 2
	OK = 3
	Error = 4
	GameMeta = 5
	GuestConnect = 6
	GuestDisconnect = 7
	Data = 8
)

type Packet struct {
	Kind PacketKind
	ConnId uint32
	Payload []byte
}

type PacketHeader struct {
	Kind PacketKind
	ConnId uint32
	Length uint32
}

func Header(p Packet) PacketHeader {
	return PacketHeader{Kind: p.Kind, ConnId: p.ConnId, Length: uint32(len(p.Payload))}
}

func StrPacket(s string) Packet {
	return Packet{Kind: Data, ConnId: 0, Payload: []byte(s)}
}

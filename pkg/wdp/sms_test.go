package wdp

import (
	"net/url"
	"reflect"
	"testing"
	"time"
)

func TestWDPGateway_parseUDH(t *testing.T) {
	udh1, _ := url.QueryUnescape("%06%05%04%23%F0%C3N")
	type args struct {
		udh []byte
	}
	tests := []struct {
		name    string
		args    args
		want    UDH
		wantErr bool
	}{
		{
			name: "Test UDH from Kannel",
			args: args{
				udh: []byte(udh1),
			},
			want: UDH{
				HeaderLen: 6,
				Ei:        5,
				EiLength:  4,
				Source:    9200,
				Dest:      49998,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &WDPGateway{}
			got, err := w.parseUDH(tt.args.udh)
			if (err != nil) != tt.wantErr {
				t.Errorf("WDPGateway.parseUDH() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("WDPGateway.parseUDH() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWDPGateway_spawnWDPConnection(t *testing.T) {
	udh1, _ := url.QueryUnescape("%06%05%04%23%F0%C3N")

	w := &WDPGateway{}
	udh, _ := w.parseUDH([]byte(udh1))

	conn, err := w.spawnWDPConnection(udh, "1234567890", 60*time.Second)
	if err != nil {
		t.Errorf("WDPGateway.spawnWDPConnection() error = %v", err)
	}
	if conn == nil {
		t.Errorf("WDPGateway.spawnWDPConnection() = %v, want not nil", conn)
	}

	time.Sleep(1 * time.Second)

	conn2, err2 := w.spawnWDPConnection(udh, "1234567890", 60*time.Second)
	if err2 != nil {
		t.Errorf("WDPGateway.spawnWDPConnection() error = %v", err2)
	}
	if conn2 == nil {
		t.Errorf("WDPGateway.spawnWDPConnection() = %v, want not nil", conn2)
	}

}

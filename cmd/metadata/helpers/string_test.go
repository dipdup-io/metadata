package helpers

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscape(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want []byte
	}{
		{
			name: "test 1",
			data: []byte(`{"name":"𝚃𝚞𝚗𝚍𝚛𝚊 𝙶𝚕𝚘𝚛𝚢","decimals":0,"description":"𝚃𝚑𝚎 𝚚𝚞𝚒𝚎𝚝𝚗𝚎𝚜𝚜 𝚍𝚘𝚎𝚜𝚗''𝚝 𝚔𝚒𝚕𝚕 𝚊𝚗𝚢𝚘𝚗\ud835, 𝚋𝚞𝚝 𝚝𝚑𝚎 𝚌𝚞𝚛𝚒𝚘𝚜𝚒𝚝𝚢 𝚍𝚘𝚎𝚜","artifactUri":"ipfs://QmbpyxoHBNWQvEATgdyy7VURZxAWZ5tYwSG56hseMe68ER/image.jpeg","displayUri":"ipfs://QmbpyxoHBNWQvEATgdyy7VURZxAWZ5tYwSG56hseMe68ER/image.jpeg","attributes":[{"name":"𝚂𝚒𝚣𝚎","value":"𝚂𝚚𝚞𝚊𝚛𝚎"},{"name":"𝙱𝚊𝚌𝚔𝚐𝚛𝚘𝚞𝚗𝚍","value":"𝚅𝚘𝚒𝚍"},{"name":"𝚃𝚢𝚙𝚎","value":"𝙲𝚘𝚕𝚕𝚎𝚌𝚝𝚒𝚋𝚕𝚎"},{"name":"𝙴𝚍𝚒𝚝𝚒𝚘𝚗","value":"25"}],"formats":[{"mimeType":"image/jpeg","fileSize":1650550,"fileName":"UpperPix20220321_232739.jpg","uri":"ipfs://QmbpyxoHBNWQvEATgdyy7VURZxAWZ5tYwSG56hseMe68ER/image.jpeg"}]}`),
			want: []byte(`{"name":"𝚃𝚞𝚗𝚍𝚛𝚊 𝙶𝚕𝚘𝚛𝚢","decimals":0,"description":"𝚃𝚑𝚎 𝚚𝚞𝚒𝚎𝚝𝚗𝚎𝚜𝚜 𝚍𝚘𝚎𝚜𝚗''𝚝 𝚔𝚒𝚕𝚕 𝚊𝚗𝚢𝚘𝚗\ud835, 𝚋𝚞𝚝 𝚝𝚑𝚎 𝚌𝚞𝚛𝚒𝚘𝚜𝚒𝚝𝚢 𝚍𝚘𝚎𝚜","artifactUri":"ipfs://QmbpyxoHBNWQvEATgdyy7VURZxAWZ5tYwSG56hseMe68ER/image.jpeg","displayUri":"ipfs://QmbpyxoHBNWQvEATgdyy7VURZxAWZ5tYwSG56hseMe68ER/image.jpeg","attributes":[{"name":"𝚂𝚒𝚣𝚎","value":"𝚂𝚚𝚞𝚊𝚛𝚎"},{"name":"𝙱𝚊𝚌𝚔𝚐𝚛𝚘𝚞𝚗𝚍","value":"𝚅𝚘𝚒𝚍"},{"name":"𝚃𝚢𝚙𝚎","value":"𝙲𝚘𝚕𝚕𝚎𝚌𝚝𝚒𝚋𝚕𝚎"},{"name":"𝙴𝚍𝚒𝚝𝚒𝚘𝚗","value":"25"}],"formats":[{"mimeType":"image/jpeg","fileSize":1650550,"fileName":"UpperPix20220321_232739.jpg","uri":"ipfs://QmbpyxoHBNWQvEATgdyy7VURZxAWZ5tYwSG56hseMe68ER/image.jpeg"}]}`),
		}, {
			name: "test 2",
			data: []byte(`{"name": "Diplomat #2785", "symbol": "TZTOP", "formats": [{"uri": "ipfs://QmWynVS7dDChU4ZWikeJ5WkoWQtJw3mTYv2nuz9xgYobw5", "mimeType": "image/png"}], "creators": ["tz1bKM4FRgAsGdDWzXs4o5HZdjBbLMbPBAA1"], "decimals": 0, "royalties": {"shares": {"tz1bKM4FRgAsGdDWzXs4o5HZdjBbLMbPBAA1": 6}, "decimals": 2}, "attributes": [{"name": "arm", "value": "Imitation Opal"}, {"name": "headdress", "value": "Face Guard"}, {"name": "katana", "value": "Uwan"}, {"name": "skin", "value": "Human Tan"}, {"name": "armor", "value": "Poor Man Armor"}, {"name": "backdrop", "value": "Grey Wave"}, {"name": "class", "value": "Commoner"}], "displayUri": "ipfs://QmWynVS7dDChU4ZWikeJ5WkoWQtJw3mTYv2nuz9xgYobw5", "artifactUri": "ipfs://QmWynVS7dDChU4ZWikeJ5WkoWQtJw3mTYv2nuz9xgYobw5", "description": "Diplomats of Tezotopia are more than a simple PFP collectible, these r@epresentatives unlock the ability to perform various diplomatic actions in the game.", "thumbnailUri": "ipfs://QmcSo4zgU8aASayU1FoVhsXVNicGch2NX2rJLHqELV4mc5", "isBooleanAmount": true}`),
			want: []byte(`{"name": "Diplomat #2785", "symbol": "TZTOP", "formats": [{"uri": "ipfs://QmWynVS7dDChU4ZWikeJ5WkoWQtJw3mTYv2nuz9xgYobw5", "mimeType": "image/png"}], "creators": ["tz1bKM4FRgAsGdDWzXs4o5HZdjBbLMbPBAA1"], "decimals": 0, "royalties": {"shares": {"tz1bKM4FRgAsGdDWzXs4o5HZdjBbLMbPBAA1": 6}, "decimals": 2}, "attributes": [{"name": "arm", "value": "Imitation Opal"}, {"name": "headdress", "value": "Face Guard"}, {"name": "katana", "value": "Uwan"}, {"name": "skin", "value": "Human Tan"}, {"name": "armor", "value": "Poor Man Armor"}, {"name": "backdrop", "value": "Grey Wave"}, {"name": "class", "value": "Commoner"}], "displayUri": "ipfs://QmWynVS7dDChU4ZWikeJ5WkoWQtJw3mTYv2nuz9xgYobw5", "artifactUri": "ipfs://QmWynVS7dDChU4ZWikeJ5WkoWQtJw3mTYv2nuz9xgYobw5", "description": "Diplomats of Tezotopia are more than a simple PFP collectible, these r@epresentatives unlock the ability to perform various diplomatic actions in the game.", "thumbnailUri": "ipfs://QmcSo4zgU8aASayU1FoVhsXVNicGch2NX2rJLHqELV4mc5", "isBooleanAmount": true}`),
		}, {
			name: "test 3",
			data: []byte(`{"name": "\u0005\u0001\u0000\u0000\u0000@339e27b6565ce3ffd5646f7b11070718bdeccc05333864da5aa3d1ed00a8a2cb"}`),
			want: []byte(`{"name": "\u0005\u0001@339e27b6565ce3ffd5646f7b11070718bdeccc05333864da5aa3d1ed00a8a2cb"}`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Escape(tt.data)
			assert.Equal(t, tt.want, got)
			if !json.Valid(got) {
				t.Errorf("invalid response JSON: %s", got)
			}
		})
	}
}

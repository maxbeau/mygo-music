package plugin

import "testing"

// Optional discovery exports cross the JS boundary and stamp every artist section.
func TestDiscoveryBridge(t *testing.T) {
	p := load(t, plain, `
 export function radio({kind,id}) {return {id:kind+":"+id,name:"Radio",public:true};}
 export function related({id}) {return {sections:[{tracks:[{id,source:"wrong"}]}]};}
 export function artist({id}) {return {artist:{id},sections:[{tracks:[{id:"song",source:"wrong"}]}]};}
 `)
	station, err := p.Radio("artist", "singer")
	if err != nil || station.ID != "artist:singer" || !station.Public {
		t.Fatalf("%+v %v", station, err)
	}
	related, err := p.Related("seed")
	if err != nil || related.Sections[0].Tracks[0].Source != "x" {
		t.Fatalf("%+v %v", related, err)
	}
	artist, err := p.Artist("singer")
	if err != nil || artist.Sections[0].Tracks[0].Source != "x" {
		t.Fatalf("%+v %v", artist, err)
	}
}

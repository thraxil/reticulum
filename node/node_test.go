package node

import (
	_ "fmt"
	"testing"
)

func Test_Hashkeys(t *testing.T) {
	n := NodeData{
		Nickname:  "test node",
		UUID:      "test-uuid",
		BaseUrl:   "localhost:8080",
		Location:  "test",
		Writeable: true,
	}
	keys := n.HashKeys()
	if len(keys) != REPLICAS {
		t.Error("not the right number of keys")
	}
	var expected = []string{
		"ae28605f0ffc34fe5314342f78efaa13ee45f699",
		"9affa344bca678572b044b50f4809e942389fbf6",
		"23360f51d95ce71902ea2b9b313de1f9c05c92a7",
		"8f6264d6b5b15840667d2414b7285a3fb7f63878",
		"e562b9e5dbfca62143230cd1e762005ffad74f8d",
		"7e212f8b753580f2e0bab7a234202c971be46626",
		"c426b974570120afd310fb9ece0c29c266f1738a", 
		"9193bc5c3ae69a053fc7dc703b6b56cd7fe65637", 
		"b9282ad8cc00462a1070e6ac7dab2c0867476f9c", 
		"9a260dc2b8804efcd77f0b634b9bf258bef2b4ca", 
		"07e025010da6c456e242d9d3d1075617aed1c4ff", 
		"f70773bc3cb0b4d7084421c3389fc58e132c9852", 
		"49cd9aa81076f95b02d2aa125d9fab1e62fa31cc", 
		"88ca97909f7cdf94f201d6b90a265157067b3430", 
		"8c37b2c35b1d5f4dcd878fe6a11f3b5a02ee62a2", 
		"fb682e05b9be61797601e60165825c0b089f755e"}
	for i := range keys {
		if keys[i] != expected[i] {
			t.Error("bad key")
		}
	}
}

func Test_Urls(t *testing.T) {
	n := NodeData{
		Nickname:  "test node",
		UUID:      "test-uuid",
		BaseUrl:   "localhost:8080",
		Location:  "test",
		Writeable: true,
	}
	var hash = "fb682e05b9be61797601e60165825c0b089f755e"
	if n.retrieveUrl(hash,"full","jpg") != "http://localhost:8080/retrieve/fb682e05b9be61797601e60165825c0b089f755e/full/jpg/" {
		t.Error("bad retrieve url")
	}
	if n.retrieveInfoUrl(hash,"full","jpg") != "http://localhost:8080/retrieve_info/fb682e05b9be61797601e60165825c0b089f755e/full/jpg/" {
		t.Error("bad retrieve info url")
	}
	if n.stashUrl() != "http://localhost:8080/stash/" {
		t.Error("bad stash url")
	}

	n.BaseUrl = "localhost:8080/"
	if n.retrieveUrl(hash,"full","jpg") != "http://localhost:8080/retrieve/fb682e05b9be61797601e60165825c0b089f755e/full/jpg/" {
		t.Error("bad retrieve url")
	}
	if n.retrieveInfoUrl(hash,"full","jpg") != "http://localhost:8080/retrieve_info/fb682e05b9be61797601e60165825c0b089f755e/full/jpg/" {
		t.Error("bad retrieve info url")
	}
	if n.stashUrl() != "http://localhost:8080/stash/" {
		t.Error("bad stash url")
	}

	n.BaseUrl = "http://localhost:8081/"
	if n.retrieveUrl(hash,"full","jpg") != "http://localhost:8081/retrieve/fb682e05b9be61797601e60165825c0b089f755e/full/jpg/" {
		t.Error("bad retrieve url")
	}
	if n.retrieveInfoUrl(hash,"full","jpg") != "http://localhost:8081/retrieve_info/fb682e05b9be61797601e60165825c0b089f755e/full/jpg/" {
		t.Error("bad retrieve info url")
	}
	if n.stashUrl() != "http://localhost:8081/stash/" {
		t.Error("bad stash url")
	}

	n.BaseUrl = "http://localhost:8081"
	if n.retrieveUrl(hash,"full","jpg") != "http://localhost:8081/retrieve/fb682e05b9be61797601e60165825c0b089f755e/full/jpg/" {
		t.Error("bad retrieve url")
	}
	if n.retrieveInfoUrl(hash,"full","jpg") != "http://localhost:8081/retrieve_info/fb682e05b9be61797601e60165825c0b089f755e/full/jpg/" {
		t.Error("bad retrieve info url")
	}
	if n.stashUrl() != "http://localhost:8081/stash/" {
		t.Error("bad stash url")
	}

}




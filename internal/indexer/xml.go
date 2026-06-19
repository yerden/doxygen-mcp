package indexer

import "encoding/xml"

type DoxygenIndex struct {
	XMLName   xml.Name        `xml:"doxygenindex"`
	Compounds []CompoundEntry `xml:"compound"`
}

type CompoundEntry struct {
	RefID string `xml:"refid,attr"`
	Kind  string `xml:"kind,attr"`
	Name  string `xml:"name"`
}

type DoxygenFile struct {
	XMLName xml.Name    `xml:"doxygen"`
	Compound CompoundDef `xml:"compounddef"`
}

type CompoundDef struct {
	ID          string      `xml:"id,attr"`
	Kind        string      `xml:"kind,attr"`
	Name        string      `xml:"compoundname"`
	Location    Location    `xml:"location"`
	BriefDesc   Description `xml:"briefdescription"`
	DetailedDesc Description `xml:"detaileddescription"`
	Members     []MemberDef `xml:"sectiondef>memberdef"`
	InnerClasses []InnerRef `xml:"innerclass"`
}

type InnerRef struct {
	RefID string `xml:"refid,attr"`
	Name  string `xml:",chardata"`
}

type MemberDef struct {
	ID         string      `xml:"id,attr"`
	Kind       string      `xml:"kind,attr"`
	Name       string      `xml:"name"`
	Type       LinkedText  `xml:"type"`
	Definition string      `xml:"definition"`
	ArgsString string      `xml:"argsstring"`
	Location   Location    `xml:"location"`
	BriefDesc  Description `xml:"briefdescription"`
	DetailDesc Description `xml:"detaileddescription"`
	Params     []Param     `xml:"param"`
}

type LinkedText struct {
	Text string `xml:",innerxml"`
}

type Location struct {
	File string `xml:"file,attr"`
	Line int    `xml:"line,attr"`
}

type Description struct {
	Para []Para `xml:"para"`
}

type Para struct {
	Text string `xml:",innerxml"`
}

type Param struct {
	Type        LinkedText  `xml:"type"`
	DeclName    string      `xml:"declname"`
	BriefDesc   Description `xml:"briefdescription"`
}

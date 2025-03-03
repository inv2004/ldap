package ldap

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	ber "github.com/go-asn1-ber/asn1-ber"
)

// scope choices
const (
	ScopeBaseObject   = 0
	ScopeSingleLevel  = 1
	ScopeWholeSubtree = 2
)

// ScopeMap contains human readable descriptions of scope choices
var ScopeMap = map[int]string{
	ScopeBaseObject:   "Base Object",
	ScopeSingleLevel:  "Single Level",
	ScopeWholeSubtree: "Whole Subtree",
}

// derefAliases
const (
	NeverDerefAliases   = 0
	DerefInSearching    = 1
	DerefFindingBaseObj = 2
	DerefAlways         = 3
)

// DerefMap contains human readable descriptions of derefAliases choices
var DerefMap = map[int]string{
	NeverDerefAliases:   "NeverDerefAliases",
	DerefInSearching:    "DerefInSearching",
	DerefFindingBaseObj: "DerefFindingBaseObj",
	DerefAlways:         "DerefAlways",
}

// NewEntry returns an Entry object with the specified distinguished name and attribute key-value pairs.
// The map of attributes is accessed in alphabetical order of the keys in order to ensure that, for the
// same input map of attributes, the output entry will contain the same order of attributes
func NewEntry(dn string, attributes map[string][]string) *Entry {
	var attributeNames []string
	for attributeName := range attributes {
		attributeNames = append(attributeNames, attributeName)
	}
	sort.Strings(attributeNames)

	var encodedAttributes []*EntryAttribute
	for _, attributeName := range attributeNames {
		encodedAttributes = append(encodedAttributes, NewEntryAttribute(attributeName, attributes[attributeName]))
	}
	return &Entry{
		DN:         dn,
		Attributes: encodedAttributes,
	}
}

// Entry represents a single search result entry
type Entry struct {
	// DN is the distinguished name of the entry
	DN string
	// Attributes are the returned attributes for the entry
	Attributes []*EntryAttribute
}

// GetAttributeValues returns the values for the named attribute, or an empty list
func (e *Entry) GetAttributeValues(attribute string) []string {
	for _, attr := range e.Attributes {
		if attr.Name == attribute {
			return attr.Values
		}
	}
	return []string{}
}

// GetEqualFoldAttributeValues returns the values for the named attribute, or an
// empty list. Attribute matching is done with strings.EqualFold.
func (e *Entry) GetEqualFoldAttributeValues(attribute string) []string {
	for _, attr := range e.Attributes {
		if strings.EqualFold(attribute, attr.Name) {
			return attr.Values
		}
	}
	return []string{}
}

// GetRawAttributeValues returns the byte values for the named attribute, or an empty list
func (e *Entry) GetRawAttributeValues(attribute string) [][]byte {
	for _, attr := range e.Attributes {
		if attr.Name == attribute {
			return attr.ByteValues
		}
	}
	return [][]byte{}
}

// GetEqualFoldRawAttributeValues returns the byte values for the named attribute, or an empty list
func (e *Entry) GetEqualFoldRawAttributeValues(attribute string) [][]byte {
	for _, attr := range e.Attributes {
		if strings.EqualFold(attr.Name, attribute) {
			return attr.ByteValues
		}
	}
	return [][]byte{}
}

// GetAttributeValue returns the first value for the named attribute, or ""
func (e *Entry) GetAttributeValue(attribute string) string {
	values := e.GetAttributeValues(attribute)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// GetEqualFoldAttributeValue returns the first value for the named attribute, or "".
// Attribute comparison is done with strings.EqualFold.
func (e *Entry) GetEqualFoldAttributeValue(attribute string) string {
	values := e.GetEqualFoldAttributeValues(attribute)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// GetRawAttributeValue returns the first value for the named attribute, or an empty slice
func (e *Entry) GetRawAttributeValue(attribute string) []byte {
	values := e.GetRawAttributeValues(attribute)
	if len(values) == 0 {
		return []byte{}
	}
	return values[0]
}

// GetEqualFoldRawAttributeValue returns the first value for the named attribute, or an empty slice
func (e *Entry) GetEqualFoldRawAttributeValue(attribute string) []byte {
	values := e.GetEqualFoldRawAttributeValues(attribute)
	if len(values) == 0 {
		return []byte{}
	}
	return values[0]
}

// Print outputs a human-readable description
func (e *Entry) Print() {
	fmt.Printf("DN: %s\n", e.DN)
	for _, attr := range e.Attributes {
		attr.Print()
	}
}

// PrettyPrint outputs a human-readable description indenting
func (e *Entry) PrettyPrint(indent int) {
	fmt.Printf("%sDN: %s\n", strings.Repeat(" ", indent), e.DN)
	for _, attr := range e.Attributes {
		attr.PrettyPrint(indent + 2)
	}
}

// Describe the tag to use for struct field tags
const decoderTagName = "ldap"

// readTag will read the reflect.StructField value for
// the key defined in decoderTagName. If omitempty is
// specified, the field may not be filled.
func readTag(f reflect.StructField) (string, bool) {
	val, ok := f.Tag.Lookup(decoderTagName)
	if !ok {
		return f.Name, false
	}
	opts := strings.Split(val, ",")
	omit := false
	if len(opts) == 2 {
		omit = opts[1] == "omitempty"
	}
	return opts[0], omit
}

// Unmarshal parses the Entry in the value pointed to by i
//
// Currently, this methods only supports struct fields of type
// string, []string, int, int64 or []byte. Other field types will not be
// regarded. If the field type is a string or int but multiple attribute
// values are returned, the first value will be used to fill the field.
//
// Example:
//
//	type UserEntry struct {
//		// Fields with the tag key `dn` are automatically filled with the
//		// objects distinguishedName. This can be used multiple times.
//		DN string `ldap:"dn"`
//
//		// This field will be filled with the attribute value for
//		// userPrincipalName. An attribute can be read into a struct field
//		// multiple times. Missing attributes will not result in an error.
//		UserPrincipalName string `ldap:"userPrincipalName"`
//
//		// memberOf may have multiple values. If you don't
//		// know the amount of attribute values at runtime, use a string array.
//		MemberOf []string `ldap:"memberOf"`
//
//		// ID is an integer value, it will fail unmarshaling when the given
//		// attribute value cannot be parsed into an integer.
//		ID int `ldap:"id"`
//
//		// LongID is similar to ID but uses an int64 instead.
//		LongID int64 `ldap:"longId"`
//
//		// Data is similar to MemberOf a slice containing all attribute
//		// values.
//		Data []byte `ldap:"data"`
//
//		// This won't work, as the field is not of type string. For this
//		// to work, you'll have to temporarily store the result in string
//		// (or string array) and convert it to the desired type afterwards.
//		UserAccountControl uint32 `ldap:"userPrincipalName"`
//	}
//	user := UserEntry{}
//	if err := result.Unmarshal(&user); err != nil {
//		// ...
//	}
func (e *Entry) Unmarshal(i interface{}) (err error) {
	// Make sure it's a ptr
	if vo := reflect.ValueOf(i).Kind(); vo != reflect.Ptr {
		return fmt.Errorf("ldap: cannot use %s, expected pointer to a struct", vo)
	}

	sv, st := reflect.ValueOf(i).Elem(), reflect.TypeOf(i).Elem()
	// Make sure it's pointing to a struct
	if sv.Kind() != reflect.Struct {
		return fmt.Errorf("ldap: expected pointer to a struct, got %s", sv.Kind())
	}

	for n := 0; n < st.NumField(); n++ {
		// Holds struct field value and type
		fv, ft := sv.Field(n), st.Field(n)

		// skip unexported fields
		if ft.PkgPath != "" {
			continue
		}

		// omitempty can be safely discarded, as it's not needed when unmarshalling
		fieldTag, _ := readTag(ft)

		// Fill the field with the distinguishedName if the tag key is `dn`
		if fieldTag == "dn" {
			fv.SetString(e.DN)
			continue
		}

		values := e.GetAttributeValues(fieldTag)
		if len(values) == 0 {
			continue
		}

		switch fv.Interface().(type) {
		case []string:
			for _, item := range values {
				fv.Set(reflect.Append(fv, reflect.ValueOf(item)))
			}
		case string:
			fv.SetString(values[0])
		case []byte:
			fv.SetBytes([]byte(values[0]))
		case int, int64:
			intVal, err := strconv.ParseInt(values[0], 10, 64)
			if err != nil {
				return fmt.Errorf("ldap: could not parse value '%s' into int field", values[0])
			}
			fv.SetInt(intVal)
		default:
			return fmt.Errorf("ldap: expected field to be of type string, []string, int, int64 or []byte, got %v", ft.Type)
		}
	}
	return
}

// NewEntryAttribute returns a new EntryAttribute with the desired key-value pair
func NewEntryAttribute(name string, values []string) *EntryAttribute {
	var bytes [][]byte
	for _, value := range values {
		bytes = append(bytes, []byte(value))
	}
	return &EntryAttribute{
		Name:       name,
		Values:     values,
		ByteValues: bytes,
	}
}

// EntryAttribute holds a single attribute
type EntryAttribute struct {
	// Name is the name of the attribute
	Name string
	// Values contain the string values of the attribute
	Values []string
	// ByteValues contain the raw values of the attribute
	ByteValues [][]byte
}

// Print outputs a human-readable description
func (e *EntryAttribute) Print() {
	fmt.Printf("%s: %s\n", e.Name, e.Values)
}

// PrettyPrint outputs a human-readable description with indenting
func (e *EntryAttribute) PrettyPrint(indent int) {
	fmt.Printf("%s%s: %s\n", strings.Repeat(" ", indent), e.Name, e.Values)
}

// SearchResult holds the server's response to a search request
type SearchResult struct {
	// Entries are the returned entries
	Entries []*Entry
	// Referrals are the returned referrals
	Referrals []string
	// Controls are the returned controls
	Controls []Control
}

// Print outputs a human-readable description
func (s *SearchResult) Print() {
	for _, entry := range s.Entries {
		entry.Print()
	}
}

// PrettyPrint outputs a human-readable description with indenting
func (s *SearchResult) PrettyPrint(indent int) {
	for _, entry := range s.Entries {
		entry.PrettyPrint(indent)
	}
}

// SearchRequest represents a search request to send to the server
type SearchRequest struct {
	BaseDN       string
	Scope        int
	DerefAliases int
	SizeLimit    int
	TimeLimit    int
	TypesOnly    bool
	Filter       string
	Attributes   []string
	Controls     []Control
}

func (req *SearchRequest) appendTo(envelope *ber.Packet) error {
	pkt := ber.Encode(ber.ClassApplication, ber.TypeConstructed, ApplicationSearchRequest, nil, "Search Request")
	pkt.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, req.BaseDN, "Base DN"))
	pkt.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagEnumerated, uint64(req.Scope), "Scope"))
	pkt.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagEnumerated, uint64(req.DerefAliases), "Deref Aliases"))
	pkt.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagInteger, uint64(req.SizeLimit), "Size Limit"))
	pkt.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagInteger, uint64(req.TimeLimit), "Time Limit"))
	pkt.AppendChild(ber.NewBoolean(ber.ClassUniversal, ber.TypePrimitive, ber.TagBoolean, req.TypesOnly, "Types Only"))
	// compile and encode filter
	filterPacket, err := CompileFilter(req.Filter)
	if err != nil {
		return err
	}
	pkt.AppendChild(filterPacket)
	// encode attributes
	attributesPacket := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "Attributes")
	for _, attribute := range req.Attributes {
		attributesPacket.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, attribute, "Attribute"))
	}
	pkt.AppendChild(attributesPacket)

	envelope.AppendChild(pkt)
	if len(req.Controls) > 0 {
		envelope.AppendChild(encodeControls(req.Controls))
	}

	return nil
}

// NewSearchRequest creates a new search request
func NewSearchRequest(
	BaseDN string,
	Scope, DerefAliases, SizeLimit, TimeLimit int,
	TypesOnly bool,
	Filter string,
	Attributes []string,
	Controls []Control,
) *SearchRequest {
	return &SearchRequest{
		BaseDN:       BaseDN,
		Scope:        Scope,
		DerefAliases: DerefAliases,
		SizeLimit:    SizeLimit,
		TimeLimit:    TimeLimit,
		TypesOnly:    TypesOnly,
		Filter:       Filter,
		Attributes:   Attributes,
		Controls:     Controls,
	}
}

// SearchWithPaging accepts a search request and desired page size in order to execute LDAP queries to fulfill the
// search request. All paged LDAP query responses will be buffered and the final result will be returned atomically.
// The following four cases are possible given the arguments:
//   - given SearchRequest missing a control of type ControlTypePaging: we will add one with the desired paging size
//   - given SearchRequest contains a control of type ControlTypePaging that isn't actually a ControlPaging: fail without issuing any queries
//   - given SearchRequest contains a control of type ControlTypePaging with pagingSize equal to the size requested: no change to the search request
//   - given SearchRequest contains a control of type ControlTypePaging with pagingSize not equal to the size requested: fail without issuing any queries
//
// A requested pagingSize of 0 is interpreted as no limit by LDAP servers.
func (l *Conn) SearchWithPaging(searchRequest *SearchRequest, pagingSize uint32) (*SearchResult, error) {
	var pagingControl *ControlPaging

	control := FindControl(searchRequest.Controls, ControlTypePaging)
	if control == nil {
		pagingControl = NewControlPaging(pagingSize)
		searchRequest.Controls = append(searchRequest.Controls, pagingControl)
	} else {
		castControl, ok := control.(*ControlPaging)
		if !ok {
			return nil, fmt.Errorf("expected paging control to be of type *ControlPaging, got %v", control)
		}
		if castControl.PagingSize != pagingSize {
			return nil, fmt.Errorf("paging size given in search request (%d) conflicts with size given in search call (%d)", castControl.PagingSize, pagingSize)
		}
		pagingControl = castControl
	}

	searchResult := new(SearchResult)
	for {
		result, err := l.Search(searchRequest)
		l.Debug.Printf("Looking for Paging Control...")
		if err != nil {
			return searchResult, err
		}
		if result == nil {
			return searchResult, NewError(ErrorNetwork, errors.New("ldap: packet not received"))
		}

		searchResult.Entries = append(searchResult.Entries, result.Entries...)
		searchResult.Referrals = append(searchResult.Referrals, result.Referrals...)
		searchResult.Controls = append(searchResult.Controls, result.Controls...)

		l.Debug.Printf("Looking for Paging Control...")
		pagingResult := FindControl(result.Controls, ControlTypePaging)
		if pagingResult == nil {
			pagingControl = nil
			l.Debug.Printf("Could not find paging control.  Breaking...")
			break
		}

		cookie := pagingResult.(*ControlPaging).Cookie
		if len(cookie) == 0 {
			pagingControl = nil
			l.Debug.Printf("Could not find cookie.  Breaking...")
			break
		}
		pagingControl.SetCookie(cookie)
	}

	if pagingControl != nil {
		l.Debug.Printf("Abandoning Paging...")
		pagingControl.PagingSize = 0
		if _, err := l.Search(searchRequest); err != nil {
			return searchResult, err
		}
	}

	return searchResult, nil
}

// Search performs the given search request
func (l *Conn) Search(searchRequest *SearchRequest) (*SearchResult, error) {
	msgCtx, err := l.doRequest(searchRequest)
	if err != nil {
		return nil, err
	}
	defer l.finishMessage(msgCtx)

	result := &SearchResult{
		Entries:   make([]*Entry, 0),
		Referrals: make([]string, 0),
		Controls:  make([]Control, 0),
	}

	for {
		packet, err := l.readPacket(msgCtx)
		if err != nil {
			return result, err
		}

		switch packet.Children[1].Tag {
		case 4:
			entry := &Entry{
				DN:         packet.Children[1].Children[0].Value.(string),
				Attributes: unpackAttributes(packet.Children[1].Children[1].Children),
			}
			result.Entries = append(result.Entries, entry)
		case 5:
			err := GetLDAPError(packet)
			if err != nil {
				return result, err
			}
			if len(packet.Children) == 3 {
				for _, child := range packet.Children[2].Children {
					decodedChild, err := DecodeControl(child)
					if err != nil {
						return result, fmt.Errorf("failed to decode child control: %s", err)
					}
					result.Controls = append(result.Controls, decodedChild)
				}
			}
			return result, nil
		case 19:
			result.Referrals = append(result.Referrals, packet.Children[1].Children[0].Value.(string))
		}
	}
}

// SearchWithChannel performs a search request and returns all search results via the given
// channel as soon as they are received. This means you get all results until an error
// happens (or the search successfully finished), e.g. for size / time limited requests all
// are recieved via the channel until the limit is reached.
func (l *Conn) SearchWithChannel(searchRequest *SearchRequest, ch chan *SearchResult) error {
	if ch == nil {
		return NewError(ErrorUsage, errors.New("ldap: SearchWithChannel got nil channel"))
	}
	defer close(ch)

	packet := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "LDAP Request")
	packet.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagInteger, l.nextMessageID(), "MessageID"))
	// encode search request
	err := searchRequest.appendTo(packet)
	if err != nil {
		return err
	}

	l.Debug.PrintPacket(packet)

	msgCtx, err := l.sendMessage(packet)
	if err != nil {
		return err
	}
	defer l.finishMessage(msgCtx)

	foundSearchResultDone := false
	for !foundSearchResultDone {
		l.Debug.Printf("%d: waiting for response", msgCtx.id)
		packetResponse, ok := <-msgCtx.responses
		if !ok {
			return NewError(ErrorNetwork, errors.New("ldap: response channel closed"))
		}
		packet, err = packetResponse.ReadPacket()
		l.Debug.Printf("%d: got response %p", msgCtx.id, packet)
		if err != nil {
			return err
		}

		if l.Debug {
			if err := addLDAPDescriptions(packet); err != nil {
				return err
			}
			ber.PrintPacket(packet)
		}

		switch packet.Children[1].Tag {
		case ApplicationSearchResultEntry:
			entry := new(Entry)
			entry.DN = packet.Children[1].Children[0].Value.(string)
			for _, child := range packet.Children[1].Children[1].Children {
				attr := new(EntryAttribute)
				attr.Name = child.Children[0].Value.(string)
				for _, value := range child.Children[1].Children {
					attr.Values = append(attr.Values, value.Value.(string))
					attr.ByteValues = append(attr.ByteValues, value.ByteValue)
				}
				entry.Attributes = append(entry.Attributes, attr)
			}
			ch <- &SearchResult{Entries: []*Entry{entry}}

		case ApplicationSearchResultDone:
			if err := GetLDAPError(packet); err != nil {
				return err
			}
			if len(packet.Children) == 3 {
				result := &SearchResult{}
				for _, child := range packet.Children[2].Children {
					decodedChild, err := DecodeControl(child)
					if err != nil {
						return fmt.Errorf("failed to decode child control: %s", err)
					}
					result.Controls = append(result.Controls, decodedChild)
				}
				ch <- result
			}
			foundSearchResultDone = true

		case ApplicationSearchResultReference:
			ref := packet.Children[1].Children[0].Value.(string)
			ch <- &SearchResult{Referrals: []string{ref}}
		}
	}

	l.Debug.Printf("%d: returning", msgCtx.id)
	return nil

}

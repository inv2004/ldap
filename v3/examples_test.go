package ldap

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"sync"
)

// This example demonstrates how to bind a connection to an ldap user
// allowing access to restricted attributes that user has access to
func ExampleConn_Bind() {
	l, err := DialURL("ldap://ldap.example.com:389")
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	err = l.Bind("cn=read-only-admin,dc=example,dc=com", "password")
	if err != nil {
		log.Fatal(err)
	}
}

// This example demonstrates how to use the search interface
func ExampleConn_Search() {
	l, err := DialURL("ldap://ldap.example.com:389")
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	searchRequest := NewSearchRequest(
		"dc=example,dc=com", // The base dn to search
		ScopeWholeSubtree, NeverDerefAliases, 0, 0, false,
		"(&(objectClass=organizationalPerson))", // The filter to apply
		[]string{"dn", "cn"},                    // A list attributes to retrieve
		nil,
	)

	sr, err := l.Search(searchRequest)
	if err != nil {
		log.Fatal(err)
	}

	for _, entry := range sr.Entries {
		fmt.Printf("%s: %v\n", entry.DN, entry.GetAttributeValue("cn"))
	}
}

func ExampleConn_SearchWithChannel() {
	l, err := DialURL(fmt.Sprintf("%s:%d", "ldap.example.com", 389))
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	searchRequest := NewSearchRequest(
		"dc=example,dc=com", // The base dn to search
		ScopeWholeSubtree, NeverDerefAliases, 0, 0, false,
		"(&(objectClass=organizationalPerson))", // The filter to apply
		[]string{"dn", "cn"},                    // A list attributes to retrieve
		nil,
	)

	// this is basically how Search() does it:
	ch := make(chan *SearchResult)
	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		for res := range ch {
			if len(res.Entries) != 0 {
				fmt.Printf("%s has DN %s\n", res.Entries[0].GetAttributeValue("cn"), res.Entries[0].DN)
			}
		}
		wg.Done()
	}()

	err = l.SearchWithChannel(searchRequest, ch)

	wg.Wait()

	if err != nil {
		log.Fatalf("Error while searching: %s", err)
	}
}

// This example demonstrates how to start a TLS connection
func ExampleConn_StartTLS() {
	l, err := DialURL("ldap://ldap.example.com:389")
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	// Reconnect with TLS
	err = l.StartTLS(&tls.Config{InsecureSkipVerify: true})
	if err != nil {
		log.Fatal(err)
	}

	// Operations via l are now encrypted
}

// This example demonstrates how to compare an attribute with a value
func ExampleConn_Compare() {
	l, err := DialURL("ldap://ldap.example.com:389")
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	matched, err := l.Compare("cn=user,dc=example,dc=com", "uid", "someuserid")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(matched)
}

func ExampleConn_PasswordModify_admin() {
	l, err := DialURL("ldap://ldap.example.com:389")
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	err = l.Bind("cn=admin,dc=example,dc=com", "password")
	if err != nil {
		log.Fatal(err)
	}

	passwordModifyRequest := NewPasswordModifyRequest("cn=user,dc=example,dc=com", "", "NewPassword")
	_, err = l.PasswordModify(passwordModifyRequest)

	if err != nil {
		log.Fatalf("Password could not be changed: %s", err.Error())
	}
}

func ExampleConn_PasswordModify_generatedPassword() {
	l, err := DialURL("ldap://ldap.example.com:389")
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	err = l.Bind("cn=user,dc=example,dc=com", "password")
	if err != nil {
		log.Fatal(err)
	}

	passwordModifyRequest := NewPasswordModifyRequest("", "OldPassword", "")
	passwordModifyResponse, err := l.PasswordModify(passwordModifyRequest)
	if err != nil {
		log.Fatalf("Password could not be changed: %s", err.Error())
	}

	generatedPassword := passwordModifyResponse.GeneratedPassword
	log.Printf("Generated password: %s\n", generatedPassword)
}

func ExampleConn_PasswordModify_setNewPassword() {
	l, err := DialURL("ldap://ldap.example.com:389")
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	err = l.Bind("cn=user,dc=example,dc=com", "password")
	if err != nil {
		log.Fatal(err)
	}

	passwordModifyRequest := NewPasswordModifyRequest("", "OldPassword", "NewPassword")
	_, err = l.PasswordModify(passwordModifyRequest)

	if err != nil {
		log.Fatalf("Password could not be changed: %s", err.Error())
	}
}

func ExampleConn_Modify() {
	l, err := DialURL("ldap://ldap.example.com:389")
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	// Add a description, and replace the mail attributes
	modify := NewModifyRequest("cn=user,dc=example,dc=com", nil)
	modify.Add("description", []string{"An example user"})
	modify.Replace("mail", []string{"user@example.org"})

	err = l.Modify(modify)
	if err != nil {
		log.Fatal(err)
	}
}

// Example_userAuthentication shows how a typical application can verify a login attempt
// Refer to https://github.com/go-ldap/ldap/issues/93 for issues revolving around unauthenticated binds, with zero length passwords
func Example_userAuthentication() {
	// The username and password we want to check
	username := "someuser"
	password := "userpassword"

	bindusername := "readonly"
	bindpassword := "password"

	l, err := DialURL("ldap://ldap.example.com:389")
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	// Reconnect with TLS
	err = l.StartTLS(&tls.Config{InsecureSkipVerify: true})
	if err != nil {
		log.Fatal(err)
	}

	// First bind with a read only user
	err = l.Bind(bindusername, bindpassword)
	if err != nil {
		log.Fatal(err)
	}

	// Search for the given username
	searchRequest := NewSearchRequest(
		"dc=example,dc=com",
		ScopeWholeSubtree, NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(objectClass=organizationalPerson)(uid=%s))", EscapeFilter(username)),
		[]string{"dn"},
		nil,
	)

	sr, err := l.Search(searchRequest)
	if err != nil {
		log.Fatal(err)
	}

	if len(sr.Entries) != 1 {
		log.Fatal("User does not exist or too many entries returned")
	}

	userdn := sr.Entries[0].DN

	// Bind as the user to verify their password
	err = l.Bind(userdn, password)
	if err != nil {
		log.Fatal(err)
	}

	// Rebind as the read only user for any further queries
	err = l.Bind(bindusername, bindpassword)
	if err != nil {
		log.Fatal(err)
	}
}

func Example_beherappolicy() {
	l, err := DialURL("ldap://ldap.example.com:389")
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	controls := []Control{}
	controls = append(controls, NewControlBeheraPasswordPolicy())
	bindRequest := NewSimpleBindRequest("cn=admin,dc=example,dc=com", "password", controls)

	r, err := l.SimpleBind(bindRequest)
	ppolicyControl := FindControl(r.Controls, ControlTypeBeheraPasswordPolicy)

	var ppolicy *ControlBeheraPasswordPolicy
	if ppolicyControl != nil {
		ppolicy = ppolicyControl.(*ControlBeheraPasswordPolicy)
	} else {
		log.Printf("ppolicyControl response not available.\n")
	}
	if err != nil {
		errStr := "ERROR: Cannot bind: " + err.Error()
		if ppolicy != nil && ppolicy.Error >= 0 {
			errStr += ":" + ppolicy.ErrorString
		}
		log.Print(errStr)
	} else {
		logStr := "Login Ok"
		if ppolicy != nil {
			if ppolicy.Expire >= 0 {
				logStr += fmt.Sprintf(". Password expires in %d seconds\n", ppolicy.Expire)
			} else if ppolicy.Grace >= 0 {
				logStr += fmt.Sprintf(". Password expired, %d grace logins remain\n", ppolicy.Grace)
			}
		}
		log.Print(logStr)
	}
}

func Example_vchuppolicy() {
	l, err := DialURL("ldap://ldap.example.com:389")
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()
	l.Debug = true

	bindRequest := NewSimpleBindRequest("cn=admin,dc=example,dc=com", "password", nil)

	r, err := l.SimpleBind(bindRequest)

	passwordMustChangeControl := FindControl(r.Controls, ControlTypeVChuPasswordMustChange)
	var passwordMustChange *ControlVChuPasswordMustChange
	if passwordMustChangeControl != nil {
		passwordMustChange = passwordMustChangeControl.(*ControlVChuPasswordMustChange)
	}

	if passwordMustChange != nil && passwordMustChange.MustChange {
		log.Printf("Password Must be changed.\n")
	}

	passwordWarningControl := FindControl(r.Controls, ControlTypeVChuPasswordWarning)

	var passwordWarning *ControlVChuPasswordWarning
	if passwordWarningControl != nil {
		passwordWarning = passwordWarningControl.(*ControlVChuPasswordWarning)
	} else {
		log.Printf("ppolicyControl response not available.\n")
	}
	if err != nil {
		log.Print("ERROR: Cannot bind: " + err.Error())
	} else {
		logStr := "Login Ok"
		if passwordWarning != nil {
			if passwordWarning.Expire >= 0 {
				logStr += fmt.Sprintf(". Password expires in %d seconds\n", passwordWarning.Expire)
			}
		}
		log.Print(logStr)
	}
}

// This example demonstrates how to use ControlPaging to manually execute a
// paginated search request instead of using SearchWithPaging.
func ExampleControlPaging_manualPaging() {
	conn, err := DialURL("ldap://ldap.example.com:389")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	var pageSize uint32 = 32
	searchBase := "dc=example,dc=com"
	filter := "(objectClass=group)"
	pagingControl := NewControlPaging(pageSize)
	attributes := []string{}
	controls := []Control{pagingControl}

	for {
		request := NewSearchRequest(searchBase, ScopeWholeSubtree, DerefAlways, 0, 0, false, filter, attributes, controls)
		response, err := conn.Search(request)
		if err != nil {
			log.Fatalf("Failed to execute search request: %s", err.Error())
		}

		// [do something with the response entries]

		// In order to prepare the next request, we check if the response
		// contains another ControlPaging object and a not-empty cookie and
		// copy that cookie into our pagingControl object:
		updatedControl := FindControl(response.Controls, ControlTypePaging)
		if ctrl, ok := updatedControl.(*ControlPaging); ctrl != nil && ok && len(ctrl.Cookie) != 0 {
			pagingControl.SetCookie(ctrl.Cookie)
			continue
		}
		// If no new paging information is available or the cookie is empty, we
		// are done with the pagination.
		break
	}
}

// This example demonstrates how to use EXTERNAL SASL with TLS client certificates.
func ExampleConn_ExternalBind() {
	var ldapCert = "/path/to/cert.pem"
	var ldapKey = "/path/to/key.pem"
	var ldapCAchain = "/path/to/ca_chain.pem"

	// Load client cert and key
	cert, err := tls.LoadX509KeyPair(ldapCert, ldapKey)
	if err != nil {
		log.Fatal(err)
	}

	// Load CA chain
	caCert, err := ioutil.ReadFile(ldapCAchain)
	if err != nil {
		log.Fatal(err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	// Setup TLS with ldap client cert
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            caCertPool,
		InsecureSkipVerify: true,
	}

	// connect to ldap server
	l, err := DialURL("ldap://ldap.example.com:389")
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	// reconnect using tls
	err = l.StartTLS(tlsConfig)
	if err != nil {
		log.Fatal(err)
	}

	// sasl external bind
	err = l.ExternalBind()
	if err != nil {
		log.Fatal(err)
	}

	// Conduct ldap queries
}

// ExampleConn_WhoAmI demonstrates how to run a whoami request according to https://tools.ietf.org/html/rfc4532
func ExampleConn_WhoAmI() {
	conn, err := DialURL("ldap.example.org:389")
	if err != nil {
		log.Fatalf("Failed to connect: %s\n", err)
	}

	_, err = conn.SimpleBind(&SimpleBindRequest{
		Username: "uid=someone,ou=people,dc=example,dc=org",
		Password: "MySecretPass",
	})
	if err != nil {
		log.Fatalf("Failed to bind: %s\n", err)
	}

	res, err := conn.WhoAmI(nil)
	if err != nil {
		log.Fatalf("Failed to call WhoAmI(): %s\n", err)
	}
	fmt.Printf("I am: %s\n", res.AuthzID)
}

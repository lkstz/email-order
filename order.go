package main

import (
	"bytes"
	"crypto/tls"
	"errors"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	"github.com/martinlindhe/base36"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/mail"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	section = &imap.BodySectionName{}

	imapAddr = os.Getenv("IMAP_ADDR")
	smtpAddr = os.Getenv("SMTP_ADDR")
	user     = os.Getenv("USER")
	pass     = os.Getenv("PASS")

	sentMbox  = os.Getenv("SENT_MBOX")
	draftMbox = os.Getenv("DRAFT_MBOX")

	waitDays int

	draftSearch *imap.SearchCriteria

	hostname string
)

type order struct {
	imapClient *client.Client
	msg        *orderMsg
}

type orderMsg struct {
	from []*mail.Address
	to   []*mail.Address
	cc   []*mail.Address
	bcc  []*mail.Address

	data []byte
}

func init() {
	rand.Seed(time.Now().UnixNano())

	// Parse wait days
	env := os.Getenv("WAIT_DAYS")
	var err error
	if waitDays, err = strconv.Atoi(env); err != nil {
		waitDays = 7
	}

	// Construct draft search criteria
	env = os.Getenv("DRAFT_SEARCH")
	parts := strings.Split(env, " ")

	c := make([]interface{}, len(parts))
	for i, p := range parts {
		c[i] = p
	}

	draftSearch = imap.NewSearchCriteria()
	if err := draftSearch.ParseWithCharset(c, nil); err != nil {
		seqSet, _ := imap.ParseSeqSet("*")
		draftSearch.Uid = seqSet
	}

	// Create hostname
	if hostname, err = os.Hostname(); err == nil {
	} else {
		hostname = "email-order"
	}
}

func PlaceOrder() error {
	o := &order{}

	defer func() error {
		if o.imapClient == nil {
			return nil
		}

		return o.imapClient.Close()
	}()

	// Check if an order can be placed
	canSend, err := o.canSend()
	if err != nil {
		return err
	}

	if !canSend {
		return nil
	}

	// Fetch draft
	draft, err := o.getDraft()
	if err != nil {
		return err
	}

	// Create a new msg from draft
	if err = o.createMsg(draft); err != nil {
		return err
	}

	// Save the new msg in sent folder
	if err := o.saveSent(); err != nil {

	}

	// Send the msg via SMTP
	if err := o.sendMsg(); err != nil {
		return err
	}

	if err := o.imapClient.Close(); err != nil {
		// TODO: Log this error but do not fail since we already sent the msg
	}

	return nil
}

func (o *order) initImap() error {
	imapClient, err := client.DialTLS(imapAddr, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return err
	}

	o.imapClient = imapClient

	if err := o.imapClient.Login(user, pass); err != nil {
		return err
	}

	return nil
}

func (o *order) canSend() (bool, error) {
	if err := o.initImap(); err != nil {
		return false, err
	}

	_, err := o.imapClient.Select(sentMbox, false)
	if err != nil {
		return false, err
	}

	s := imap.NewSearchCriteria()
	s.Since = time.Now().AddDate(0, 0, waitDays*-1)
	ids, err := o.imapClient.Search(s)
	if err != nil {
		return false, err
	}

	if len(ids) < 1 {
		return true, nil
	}

	if err := o.imapClient.Close(); err != nil {
		return false, err
	}

	return false, nil
}

func (o *order) getDraft() (*imap.Message, error) {
	if err := o.initImap(); err != nil {
		return nil, err
	}

	_, err := o.imapClient.Select(draftMbox, true)
	if err != nil {
		return nil, err
	}

	ids, err := o.imapClient.UidSearch(draftSearch)
	if err != nil {
		return nil, err
	}

	if len(ids) < 1 {
		return nil, errors.New("no draft template found in drafts folder")
	}

	seqSet := new(imap.SeqSet)
	seqSet.AddNum(ids[0])

	messages := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	go func() {
		done <- o.imapClient.UidFetch(seqSet, []imap.FetchItem{section.FetchItem()}, messages)
	}()

	var msg *imap.Message
	for m := range messages {
		msg = m
	}

	if err := <-done; err != nil {
		return nil, err
	}

	if err := o.imapClient.Close(); err != nil {
		// TODO: Log this error but do not fail since we already have the msg
	}

	return msg, nil
}

func (o *order) createMsg(draft *imap.Message) error {
	literal := draft.GetBody(section)

	msg := &orderMsg{}

	m, err := message.Read(literal)
	if message.IsUnknownEncoding(err) {
		log.Println("Unknown encoding:", err)
	} else if err != nil {
		log.Fatal(err)
	}

	// Get header fields
	if msg.from, err = getAddress(m.Header, "From"); err != nil {
		return err
	}

	if msg.to, err = getAddress(m.Header, "To"); err != nil {
		return err
	}

	if msg.cc, err = getAddress(m.Header, "Cc"); err != nil {
		return err
	}

	if msg.bcc, err = getAddress(m.Header, "Bcc"); err != nil {
		return err
	}

	// Remove bcc from message header
	if msg.bcc != nil && len(msg.bcc) > 0 {
		m.Header.Del("Bcc")
	}

	// Set date header to current date
	m.Header.Set("Date", time.Now().Format(time.RFC822Z))

	// Set message id header
	m.Header.Set("Message-ID", getMessageId())

	var b bytes.Buffer
	mw, err := message.CreateWriter(&b, m.Header)
	if err != nil {
		log.Fatal(err)
	}

	// Copy message parts to writer
	var cp func(w *message.Writer, e *message.Entity) error
	cp = func(w *message.Writer, e *message.Entity) error {
		if mr := e.MultipartReader(); mr != nil {
			// This is a multipart entity, cp each of its parts
			for {
				p, err := mr.NextPart()
				if err == io.EOF {
					break
				} else if err != nil {
					return err
				}

				pw, err := w.CreatePart(p.Header)
				if err != nil {
					return err
				}

				if err := cp(pw, p); err != nil {
					return err
				}

				if err := pw.Close(); err != nil {
					return err
				}
			}
			return nil
		} else {
			_, err := io.Copy(w, e.Body)
			return err
		}
	}

	if err := cp(mw, m); err != nil {
		return err
	}

	if err := mw.Close(); err != nil {
		return err
	}

	if msg.data, err = ioutil.ReadAll(&b); err != nil {
		return err
	}

	o.msg = msg
	return nil
}

func getAddress(header message.Header, field string) ([]*mail.Address, error) {
	v := header.Get(field)

	if len(v) == 0 {
		return nil, nil
	}

	return mail.ParseAddressList(header.Get(field))
}

// Generates a message id according to https://tools.ietf.org/html/draft-ietf-usefor-message-id-01
func getMessageId() string {
	var sb strings.Builder

	sb.WriteRune('<')

	// Add current time
	t := []byte(time.Now().Format("20060102030405.000"))
	sb.WriteString(base36.EncodeBytes(t))
	sb.WriteRune('.')

	// Generate random data
	b := make([]byte, 64)
	rand.Read(b)
	sb.WriteString(base36.EncodeBytes(b))
	sb.WriteRune('@')

	// Write hostname
	sb.WriteString(hostname)

	sb.WriteRune('>')

	return sb.String()
}

func (o *order) saveSent() error {
	if err := o.initImap(); err != nil {
		return err
	}

	buf := bytes.NewBuffer(o.msg.data)

	if err := o.imapClient.Append(sentMbox, []string{"\\Seen"}, time.Now(), buf); err != nil {
		return err
	}

	return nil
}

func (o *order) sendMsg() error {
	auth := sasl.NewPlainClient("", user, pass)

	c, err := smtp.DialTLS(smtpAddr, nil)
	if err != nil {
		return err
	}
	defer c.Close()

	if err = c.Hello(hostname); err != nil {
		return err
	}

	if err = c.Auth(auth); err != nil {
		return err
	}

	// Send from
	for _, from := range o.msg.from {
		if err = c.Mail(from.Address); err != nil {
			return err
		}

		break
	}

	// Send recipients
	for _, to := range o.msg.to {
		if err = c.Rcpt(to.Address); err != nil {
			return err
		}
	}

	for _, cc := range o.msg.cc {
		if err = c.Rcpt(cc.Address); err != nil {
			return err
		}
	}

	for _, bcc := range o.msg.bcc {
		if err = c.Rcpt(bcc.Address); err != nil {
			return err
		}
	}

	// Send data
	w, err := c.Data()
	if err != nil {
		return err
	}

	if _, err = bytes.NewBuffer(o.msg.data).WriteTo(w); err != nil {
		return err
	}

	if err = w.Close(); err != nil {
		return err
	}

	if err = c.Quit(); err != nil {
		return err
	}

	return nil
}

# email-order

A small tool that sends an email from a draft.

This tool was created for AWS Lambda function being invoked by an [AWS IoT button](https://aws.amazon.com/iotbutton/).
The button's current purpose is to send an email to the local beverage dealer to order new beverages.

A few configuration values aside, this tool does not depend on any data stored in S3 or something else.
The message template and the last send date are fetched via IMAP to allow simple adjustments on the template.

## Workflow

1. Check if any messages, younger than a configurable timeout, are placed in the sent folder. If there are, then no new mail is sent.
2. Get a draft from draft folder
3. Adjust some headers in the draft message
4. Store the message in sent folder
5. Send the message

## Configuration

The configuration of this tool is done by setting environment variable values.

### `IMAP_ADDR`

Defines the IMAP server address and port.
Currently, plain IMAP is not supported.

_Example:_

```bash
export IMAP_ADDR=imap.host.com:993
```

### `SMTP_ADDR`

Defines the SMTP server address and port.
Currently, plain SMTP is not supported.

_Example:_

```bash
export SMTP_ADDR=smtp.host.com:465
```

### `USER`

The username to use for IMAP and SMTP login.

_Example:_

```bash
export USER=user@host.com
```

### `PASS`

The password to use for IMAP and SMTP login.

_Example:_

```bash
export PASS=s3cr3tpassw0rd
```

### `SENT_MBOX`

The name of the mailbox that contains sent items.

_Example:_

```bash
export SENT_MBOX=Sent
```

### `DRAFT_MBOX`

The name of the mailbox that contains draft items.

_Example:_

```bash
export DRAFT_MBOX=Drafts
```

### `WAIT_DAYS`

The number of days to wait until allowing the next email to be sent.

_Example:_

```bash
export WAIT_DAYS=7
```

### `DRAFT_SEARCH`

The IMAP search key to filter for a draft.

_Example:_

```bash
export DRAFT_SEARCH=TO recipient@company.com
```

## Todo

- [ ] Use `SPECIAL-USE` to get sent and draft folder names
- [x] Make send interval configurable
- [ ] Enhance documentation
- [ ] Allow specification of sent search key

## License

GNU General Public License v3.0

See [LICENSE](https://github.com/kolletzki/email-order/blob/master/LICENSE) for full license text.

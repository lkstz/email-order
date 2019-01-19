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

This helped me receive mail on a domain that I did not have a real
IMAP mail server for. I was using it just for sending mail with AWS
SES. But I also wanted to receive emails if someone replied to a sent
email.

First follow the instructions in AWS's documentation on
[setting up email receiving](https://docs.aws.amazon.com/ses/latest/dg/receiving-email-mx-record.html). Make sure to add the MX record on your domain:

```
10 inbound-smtp.<regionInboundUrl>.amazonaws.com
```

Then in the AWS SES console, I created a new email receiving rule on
the domain with the action "Deliver to Amazon S3 bucket". Create a new
bucket. All incoming emails will be dropped in that bucket. We can use
the PutObject events to trigger this Lambda.

# Usage

## 1. Create a new IAM policy

Make it allow `s3:GetObject` and `ses:SendRawEmail`.

Example (change `<bucketName>`, `<region>`, `<awsAccountId>`):

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "VisualEditor0",
            "Effect": "Allow",
            "Action": [
                "logs:CreateLogStream",
                "logs:CreateLogGroup",
                "logs:PutLogEvents"
            ],
            "Resource": "*"
        },
        {
            "Sid": "VisualEditor1",
            "Effect": "Allow",
            "Action": [
                "s3:GetObject",
                "ses:SendRawEmail"
            ],
            "Resource": [
                "arn:aws:s3:::<bucketName>/*",
                "arn:aws:ses:<region>:<awsAccountId>:identity/*"
            ]
        }
    ]
}
```

## 2. Create lambda function and configure environment variables

`FORWARD_TO_ADDRESS`: the email address you want to forward incoming mail to

## 3. Build this code

```
make bootstrap
```

(Prerequisites: Install [Go](https://go.dev/))


You should see `lambda.zip` in the directory. Upload the ZIP to AWS Lambda an deploy the Lambda.

## 3. Add a trigger for the Lambda

Set the trigger to Put events on the S3 bucket that SES puts incoming emails in.

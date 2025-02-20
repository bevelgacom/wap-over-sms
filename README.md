# WAP over SMS Gateway

This program is meant as a sidecar tool for [Kannel](https://www.kannel.org/) to support the WDP over SMS Protocol as defined in 
[Wireless Application Protocol -Wireless Datagram Protocol Specification](https://www.wapforum.org/what/technical/wdp-30-apr-98.pdf)
and implemented in a few devices such as the Nokia 7110.

It works as a go-between between the `smsbox` and the `wapbox` parts of Kannel to receive and send WDP data over SMS.
The design is that one `wap-over-sms` is bound to one GSM modem/smsbox instance.

## Credits
- [Manawyrm](https://github.com/Manawyrm) for the [original PHP implementation used to build this one!](https://discourse.osmocom.org/t/wap-over-sms-experiments-nokia-7110/414)

## Why?

With GPRS phones only appearing around 2002 there is 3 years of CSD capable only phones being released.
CSD is being turned off on large scale due to the need of extra modem infrastructure and 2G timeslots.

SMS is a perfect alternate bearer especially with the availability of cheap unlimited SMS plans. While it is a very slow method
compared to CSD or (comparable very fast) GPRS it does keep the first generation of WAP phones connected to the internet.

## Setup

You need both [Kannel](https://www.kannel.org/) and a `wap-over-sms` setup.

### Kannel (modem configuration is out of scope)

```
group = core
admin-port = 13000
admin-password = bar
admin-deny-ip = "*.*.*.*"
smsbox-port = 13003
wapbox-port = 13002
wdp-interface-name = "*"
log-file = "/var/log/kannel/bearerbox.log"
box-deny-ip = "*.*.*.*"
box-allow-ip = "127.0.0.1"

group = wapbox
bearerbox-host = 127.0.0.1
syslog-level = none

group = smsc
[... see Kannel docs]

group = smsbox
bearerbox-host = localhost
sendsms-port = 13014

group = sendsms-user
username = <USERNAME>
password = <PASSWORD>

group = sms-service
keyword = default
post-url = "http://localhost:8089/kannel?token=<TOKEN>"
send-sender = 1
max-messages = 0
catch-all = true
omit-empty = true
```

### wap-over-sms

```
wap-over-sms serve \
      --smsbox-host "127.0.0.1:13014" \
      --smsbox-username "<USERNAME>" \
      --smsbox-password "<PASSWORD>" \
      --smsbox-sender-msn "<YOUR MODEMS NUMBER IN INTERNATIONAL FORMAT>" \
      --token "<TOKEN>"
```

## Supported Phones

- [Nokia 7110](https://en.wikipedia.org/wiki/Nokia_7110)
- Probably others, this is a rare WAP bearer if you know another model please let us know!

## Good to know when running on the public GSM network

Some GSM carriers will block outgoing SMSes with WDP headers. The reason for this is unknown, a suspicion is that it is related to a legacy billing system trying
to charge for a WAP session and failing. It is reccomended to try a few phone operators when you have issues.
Some operators also drop these SMSes when roaming while forwarding them on their own network, reason behind this is unknown.
# Firmware CI Contest Client

This repo shall contain the contest client which is part of the FirmwareCI
Docker Image. Additionally, it also contains the tests that can be run.

## Contest Client

The contest client shall take a JSON as input which specifies what to run. We
need to check how to work around the fact that we need templating before we
actually start the job. Also we need to know the target before starting the
job. In my head it could flow like that:

* API -> ConTest Client
* ConTest Client uses Golang templating to choose the correct job and fills in
the target. Probably we need to adjust more than only the target itself.
*Better*: ConTest already receives the full Job Description .yaml file. This has
the upside that job descriptions can be changed on the server - and will
automatically pur into clients/servers - instead of having the server update
repos. The traffic overhead can be ignored for now - we probably need a better
solution at some point in time.
* ConTest Client pulls on the result. Is there a better solution than just
  polling? Polling is slim - however puts load on the servers as we are
constantly polling the status, what if polling errors out? We need some proper
*error handling* here.
* Once we receive the result we push this to some internal API (external to this
client) and save the results there. Additionally we activate LogStash/Beats on
the contest server to get all the logs. Thus we elimate the fact to do this with
the client. Do we use LogStash/Beats as our main logging service? Should we
additionally also push the report? Where do we do the parsing and formating?

Done. Easy.

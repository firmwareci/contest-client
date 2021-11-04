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
* ConTest Client pulls on the result
* Once we receive the result we push this to some internal API (external to this
client) and save the results there. Additionally we activate LogStash/Beats on
the contest server to get all the logs. Thus we elimate the fact to do this with
the client.

Done. Easy.

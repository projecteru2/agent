Deploy Eru-Agent With Systemd
=====

You can use the file `eru-agent.service` to create systemd service.

Restart with Systemd
-----

If your systemd support `RestartKillSignal=`, simply use `systemctl restart eru-agent`.

If not, please make sure to send `SIGUSR1` signal to stop eru-agent first, then `systemctl start eru-agent`.  
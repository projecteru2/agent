FROM hub.ricebook.net/base/alpine:base-2017.03.14
COPY agent /usr/bin/eru-agent
RUN mkdir /etc/eru/
COPY agent.yaml.sample /etc/eru/agent.yaml.sample
CMD eru-agent

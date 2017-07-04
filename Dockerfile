FROM alpine
COPY agent /usr/bin/eru-agent
RUN mkdir /etc/eru/
COPY agent.yaml.sample /etc/eru/agent.yaml.sample
CMD eru-agent

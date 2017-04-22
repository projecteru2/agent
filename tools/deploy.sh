#!/bin/bash


set -e
deploy_mode=${1-test}
if [ $deploy_mode == "test" ]
then
  ssh c1-docker-1 << EOF
  sudo yum --enablerepo=ricebook clean metadata
  sudo yum makecache fast
  sudo yum remove -y eru-agent
  sudo yum install -y eru-agent
  sudo systemctl daemon-reload
  sudo systemctl restart eru-agent.service
EOF
  ssh c1-docker-2 << EOF
  sudo yum --enablerepo=ricebook clean metadata
  sudo yum makecache fast
  sudo yum remove -y eru-agent
  sudo yum install -y eru-agent
  sudo systemctl daemon-reload
  sudo systemctl restart eru-agent.service
EOF
else
  echo "To deploy eru-agent in production environment, use ansible"
  exit 127
fi

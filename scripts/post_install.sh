#!/usr/bin/env bash

echo "Linking new version"

if [ -d /data/errplane-agent/plugins ]; then
    mv /data/errplane-agent/plugins/* /data/errplane-agent/shared/plugins/
    rmdir /data/errplane-agent/plugins/
fi
ln -sfn /data/errplane-agent/versions/REPLACE_VERSION /data/errplane-agent/current
ln -sfn /data/errplane-agent/current/agent /usr/bin/errplane-agent
ln -sfn /data/errplane-agent/current/agent_ctl /usr/bin/errplane-agent_ctl
ln -sfn /data/errplane-agent/current/errplane-agent-daemon /usr/bin/errplane-agent-daemon
ln -sfn /data/errplane-agent/current/config-generator /usr/bin/errplane-config-generator
ln -sfn /data/errplane-agent/current/sudoers-generator /usr/bin/errplane-sudoers-generator
ln -sfn /data/errplane-agent/shared/log.txt /data/errplane-agent/current/log.txt

chown errplane:errplane -R /data/errplane-agent/current
chown errplane:errplane -R /usr/bin/errplane-agent

if which update-rc.d > /dev/null 2>&1 ; then
    update-rc.d -f errplane-agent remove
    update-rc.d errplane-agent defaults
else
    chkconfig --add errplane-agent
fi

echo "Finished updating the Errplane agent"

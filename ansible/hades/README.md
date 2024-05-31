Hades CI
=========

Currently this role deploys redis on the same host as the hades API. 


Requirements
------------

This role expexts the following packages to be installed on the target machine:

- docker

Role Variables
--------------

See `defaults/main.yml` for the list of variables and their default values.

Dependencies
------------

A list of other roles hosted on Galaxy should go here, plus any details in regards to parameters that may need to be set for other roles, or variables that are used from other roles.

Example Playbook
----------------
  
```yaml
- name: Setup hades scheduler
  hosts: hades_dev_scheduler
  roles: 
    - role: hades
      vars: 
        hades_version: "latest"
        hades_node_role: "scheduler"
        hades_redis_host: "redis.hades.example"
        hades_redis_password: "redis_password"


- name: Setup hades api 
  hosts: hades_dev_api
  roles: 
    - role: hades
      vars: 
        hades_version: "latest"
        hades_node_role: "api"
        hades_api_certificate_fullchain_path: "/var/lib/cert/cert.fullchain.pem"
        hades_api_certificate_key_path: "/var/lib/cert/cert.privkey.pem"
        hades_redis_password: "redis_password"
```

# frozen_string_literal: true

require 'rspec'
require 'bosh/template/test'
require_relative 'spec_helper'

describe 'log-cache-syslog-server job' do
  let(:release_dir) { File.join(File.dirname(__FILE__), '..') }
  let(:release) { Bosh::Template::Test::ReleaseDir.new(release_dir) }
  let(:job) { release.job('log-cache-syslog-server') }

  describe 'syslog_client_ca.crt' do
    let(:template) { job.template('config/certs/syslog_client_ca.crt') }

    it 'can render the template' do
      properties = {
        'syslog_client_ca_cert' => 'syslog_client_ca_cert'
      }

      actual = template.render(properties)
      expect(actual).to match('syslog_client_ca_cert')
    end
  end

  describe 'syslog.crt' do
    let(:template) { job.template('config/certs/syslog.crt') }

    it 'can render the template' do
      properties = {
        'tls' => {
          'cert' => 'cert'
        }
      }

      actual = template.render(properties)
      expect(actual).to match('cert')
    end
  end

  describe 'syslog.key' do
    let(:template) { job.template('config/certs/syslog.key') }

    it 'can render the template' do
      properties = {
        'tls' => {
          'key' => 'key'
        }
      }

      actual = template.render(properties)
      expect(actual).to match('key')
    end
  end

  describe 'bpm.yml' do
    let(:template) { job.template('config/bpm.yml') }
    let(:links) do
      [
        Bosh::Template::Test::Link.new(
          name: 'log-cache',
          properties: {
            'port' => 8080
          }
        )
      ]
    end

    it 'contains the tls configuration for the syslog server' do
      certPath = "/var/vcap/jobs/log-cache-syslog-server/config/certs"
      properties = {
        'syslog_client_ca_cert' => "#{certPath}/syslog_client_ca.crt"
      }
      bpm_yml = YAML.safe_load(template.render(properties, consumes: links))
      env = bpm_process(bpm_yml, 0)['env']

      expect(env).to include('SYSLOG_CLIENT_TRUSTED_CA_FILE')
      expect(env).to include('SYSLOG_TLS_CERT_PATH')
      expect(env).to include('SYSLOG_TLS_KEY_PATH')
      expect(env.fetch("SYSLOG_CLIENT_TRUSTED_CA_FILE")).to eq("#{certPath}/syslog_client_ca.crt")
      expect(env.fetch("SYSLOG_TLS_CERT_PATH")).to eq("#{certPath}/syslog.crt")
      expect(env.fetch("SYSLOG_TLS_KEY_PATH")).to eq("#{certPath}/syslog.key")
    end
  end
end

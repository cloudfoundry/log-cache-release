# frozen_string_literal: true

require 'rspec'
require 'bosh/template/test'
require_relative 'spec_helper'

describe 'log-cache-nozzle job' do
  let(:release_dir) { File.join(File.dirname(__FILE__), '..') }
  let(:release) { Bosh::Template::Test::ReleaseDir.new(release_dir) }
  let(:job) { release.job('log-cache-nozzle') }

  describe 'metrics_ca.crt' do
    let(:template) { job.template('config/certs/metrics_ca.crt') }

    it 'has correctly processed the ca certificate template' do
      expected = 'helloworld'

      properties = {
        'metrics' => {
          'ca_cert' => 'helloworld'
        }
      }

      actual = template.render(properties)
      expect(actual).to match(expected)
    end

    it 'has correctly processed with no properties' do
      expected = ''

      actual = template.render({})
      expect(actual).to match(expected)
    end
  end

  describe 'metrics.crt' do
    let(:template) { job.template('config/certs/metrics.crt') }

    it 'has correctly processed the certificate template' do
      expected = 'helloworld'

      properties = {
        'metrics' => {
          'cert' => 'helloworld'
        }
      }

      actual = template.render(properties)
      expect(actual).to match(expected)
    end

    it 'has correctly processed with no properties' do
      expected = ''

      actual = template.render({})
      expect(actual).to match(expected)
    end
  end

  describe 'metrics.key' do
    let(:template) { job.template('config/certs/metrics.key') }

    it 'has correctly processed the key template' do
      expected = 'helloworld'

      properties = {
        'metrics' => {
          'key' => 'helloworld'
        }
      }

      actual = template.render(properties)
      expect(actual).to match(expected)
    end

    it 'has correctly processed with no properties' do
      expected = ''

      actual = template.render({})
      expect(actual).to match(expected)
    end
  end

  describe 'prom_scraper_config.yml' do
    let(:template) { job.template('config/prom_scraper_config.yml') }

    it 'is a empty file when not enabled' do
      properties = {
        'enabled' => false
      }

      actual = template.render(properties)
      expect(actual).to eq("\n")
    end
    it 'renders correctly when enabled' do
      properties = {
        'enabled' => true,
        'metrics' => {
          'port' => 55,
          'server_name' => 'server-name'
        }
      }
      actual = YAML.safe_load(template.render(properties))
      expect(actual['port']).to equal(55)
      expect(actual['server_name']).to eq("server-name")
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
        ),
        Bosh::Template::Test::Link.new(
          name: 'reverse_log_proxy',
          instances: [Bosh::Template::Test::LinkInstance.new(address: 'rlp.is.here')],
          properties: {
            'reverse_log_proxy' => { 'egress' => { 'port' => 8181 } }
          }
        )
      ]
    end

    it 'does not contain metrics cert env vars when no metrics CA cert is provided' do
      bpm_yml = YAML.safe_load(template.render({}, consumes: links))
      env = bpm_process(bpm_yml, 0)['env']
      expect(env).to_not include('METRICS_CA_FILE_PATH')
      expect(env).to_not include('METRICS_CERT_FILE_PATH')
      expect(env).to_not include('METRICS_KEY_FILE_PATH')
    end

    it 'does contains metrics cert env vars when provided' do
      properties = {
        'metrics' => {
          'ca_cert' => 'some_ca',
          'cert' => 'some_cert',
          'key' => 'some_key'
        }
      }

      bpm_yml = YAML.safe_load(template.render(properties, consumes: links))
      env = bpm_process(bpm_yml, 0)['env']
      expect(env).to include('METRICS_CA_FILE_PATH')
      expect(env).to include('METRICS_CERT_FILE_PATH')
      expect(env).to include('METRICS_KEY_FILE_PATH')
    end
  end
end

# cmd-exclude-prefixes-k8s
Prefix service is designed to collect the local IP address ranges. Additionally, user defined address ranges could be configured to be treated as local. 

# Usage

## Environment config

* `NSM_EXCLUDED_PREFIXES`       - List of excluded prefixes
* `NSM_CONFIG_MAP_NAMESPACE`    - Namespace of user config map (default: "default")
* `NSM_CONFIG_MAP_NAME`         - Name of user config map (default: "excluded-prefixes-config")
* `NSM_CONFIG_MAP_KEY`          - key in the input configmap by which we retrieve data('filename' in data section in configmap specification yaml file) (default: "excluded_prefixes_input.yaml")
* `NSM_OUTPUT_CONFIG_MAP_NAME`  - Name of nsm config map (default: "nsm-config")
* `NSM_OUTPUT_CONFIG_MAP_KEY`   - key in the output configmap by which we retrieve data('filename' in data section in configmap specification yaml file) (default: "excluded_prefixes_output.yaml")
* `NSM_OUTPUT_FILE_PATH`        - Path of output prefixes file (default: "/var/lib/networkservicemesh/config/excluded_prefixes.yaml")
* `NSM_PREFIXES_OUTPUT_TYPE`    - Where to write excluded prefixes (default: "file")
* `NSM_LOG_LEVEL`               - Log level (default: "INFO")
* `NSM_OPEN_TELEMETRY_ENDPOINT` - OpenTelemetry Collector Endpoint (default: "otel-collector.observability.svc.cluster.local:4317")
* `NSM_METRICS_EXPORT_INTERVAL` - interval between mertics exports (default: "10s")

package docker

import (
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	dockerClientPkg "github.com/docker/docker/client"
	"github.com/opctl/opctl/sdks/go/model"
	uuid "github.com/satori/go.uuid"

	"net"
	"os"
	"strconv"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/glinton/ping"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const (
	UTUN = "utun"
)

var (
	defaultKey     wgtypes.Key
	listenPort     = 3333
	hostPrivateKey *wgtypes.Key
	vmPeerIp       = "10.33.33.2"
	vmPrivateKey   *wgtypes.Key
	hostPeerIp     = "10.33.33.1"
	hostPeerCIDR   = hostPeerIp + "/32"
	// wgUpMutex serializes wgUp. It runs from a goroutine on every container
	// start, so without this, concurrent starts race to create utun devices and
	// bind the WireGuard host socket on :listenPort.
	wgUpMutex sync.Mutex
)

// ensureNetworkAttached is concurrency safe and is intended to be run every time a container starts in order to self-heal
// in cases such as Docker For mac power saver mode activating. Power saver mode is enabled in docker by default and shuts
// the VM off after 5 min of no containers being run.
func ensureNetworkAttached(
	ctx context.Context,
	dockerClient dockerClientPkg.CommonAPIClient,
) error {

	// Routes to container IPs do not exist on docker for mac. This is inconsistent with linux docker
	// and is so important we implement this bandaid.
	//
	// note: This will likely be brittle because It's relying on undocumented docker for mac internals.
	if runtime.GOOS == "darwin" {
		inspectCtx, cancelInspect := withDockerTimeout(ctx, dockerInspectTimeout())
		var networkInspect network.Inspect
		networkInspectErr := instrumentedDockerCall("NetworkInspect", networkName, func() error {
			var err error
			networkInspect, err = dockerClient.NetworkInspect(
				inspectCtx,
				networkName,
				network.InspectOptions{},
			)
			return err
		})
		cancelInspect()
		if networkInspectErr != nil {
			return fmt.Errorf("unable to inspect network: %w", networkInspectErr)
		}

		if gwm, _ := networkInspect.Options[gatewayModeIpV4]; gwm != natUnprotected {
			// recreate network if gateway_mode_ipv4 not nat-unprotected
			removeCtx, cancelRemove := withDockerTimeout(ctx, dockerMutationTimeout())
			err := instrumentedDockerCall("NetworkRemove", networkName, func() error {
				return dockerClient.NetworkRemove(removeCtx, networkName)
			})
			cancelRemove()
			if err != nil {
				return err
			}

			return ensureNetworkExists(ctx, dockerClient, networkName)
		}

		go func() {
			defer func() {
				if panic := recover(); panic != nil {
					// recover from panics
					fmt.Printf("recovered from panic: %s\n%s\n", panic, string(debug.Stack()))
				}
			}()

			err := wgUp(dockerClient, networkInspect)
			if err != nil {
				fmt.Println(err.Error())
			}
		}()
	}

	return nil
}

func wgUp(
	dockerClient dockerClientPkg.CommonAPIClient,
	network network.Inspect,
) error {
	// Serialize so concurrent container starts don't race on key generation,
	// utun creation, or the :listenPort bind below.
	wgUpMutex.Lock()
	defer wgUpMutex.Unlock()

	if hostPrivateKey == nil {
		pk, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return fmt.Errorf("Failed to generate host private key: %w", err)
		}
		hostPrivateKey = &pk
	}

	if vmPrivateKey == nil {
		pk, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return fmt.Errorf("Failed to generate VM private key: %w", err)
		}
		vmPrivateKey = &pk
	}

	go func() {
		defer func() {
			if panic := recover(); panic != nil {
				// recover from panics
				fmt.Printf("recovered from panic: %s\n%s\n", panic, string(debug.Stack()))
			}
		}()

		ctx := context.Background()
		pingCtx, _ := context.WithTimeout(ctx, time.Second)
		// test if VM pingable
		_, err := ping.IPv4(pingCtx, vmPeerIp)
		// if VM pingable, nothing to do
		if err == nil {
			return
		}

		if err := setupVm(
			ctx,
			dockerClient,
			listenPort,
			hostPeerIp,
			vmPeerIp,
			*hostPrivateKey,
			*vmPrivateKey,
		); err != nil {
			fmt.Println(err.Error())
		}
	}()

	tunDevice, err := createTunIfNotExists(context.Background())
	if err != nil {
		return err
	}

	if tunDevice == nil {
		// utunDevice alread exists; nothing to do...
		return nil
	}

	interfaceName, err := tunDevice.Name()
	if err != nil {
		return err
	}

	fileUAPI, err := ipc.UAPIOpen(interfaceName)
	if err != nil {
		return fmt.Errorf("UAPI listen error: %w", err)
	}

	errs := make(chan error)

	uapi, err := ipc.UAPIListen(interfaceName, fileUAPI)
	if err != nil {
		return fmt.Errorf("Failed to listen on UAPI socket: %w", err)
	}

	// Clean up
	defer uapi.Close()

	wgDevice := device.NewDevice(
		tunDevice,
		conn.NewDefaultBind(),
		device.NewLogger(
			1,
			fmt.Sprintf("(%s) ", interfaceName),
		),
	)

	go func() {
		defer func() {
			if panic := recover(); panic != nil {
				// recover from panics
				fmt.Printf("recovered from panic: %s\n%s\n", panic, string(debug.Stack()))
			}
		}()

		for {
			conn, err := uapi.Accept()
			if err != nil {
				errs <- err
				return
			}
			// Wrap IpcHandle in a panic recoverer. wgDevice.IpcHandle is a
			// vendored WireGuard call that processes a UAPI socket connection
			// — a malformed message or any unrecovered panic inside it would
			// otherwise take down the whole daemon process. Yesterday's
			// "daemon vanished mid-op, containers orphaned" symptom is exactly
			// what that would look like. Log + stack so we have evidence for
			// next time, but keep the daemon alive.
			go func(conn net.Conn) {
				defer func() {
					if panicValue := recover(); panicValue != nil {
						// route through the stdlib logger (redirected to the daemon's
						// rotating log + stderr) rather than fmt.Printf to stdout, which
						// may be /dev/null for a detached daemon.
						log.Printf("[opctl docker] recovered from wireguard IpcHandle panic: %s\n%s", panicValue, string(debug.Stack()))
					}
				}()
				wgDevice.IpcHandle(conn)
			}(conn)
		}
	}()

	_, wildcardIpNet, err := net.ParseCIDR("0.0.0.0/0")
	if err != nil {
		return fmt.Errorf("Failed to parse wildcard CIDR: %w", err)
	}

	_, vmIpNet, err := net.ParseCIDR(vmPeerIp + "/32")
	if err != nil {
		return fmt.Errorf("Failed to parse VM peer CIDR: %w", err)
	}

	wgClient, err := wgctrl.New()
	if err != nil {
		return fmt.Errorf("Failed to create new wgctrl client: %w", err)
	}

	defer wgClient.Close()

	peer := wgtypes.PeerConfig{
		PublicKey: vmPrivateKey.PublicKey(),
		AllowedIPs: []net.IPNet{
			*wildcardIpNet,
			*vmIpNet,
		},
	}

	// Setting ListenPort opens the WireGuard host UDP socket on :listenPort.
	// After a quick daemon restart the previous daemon's socket may not be
	// released yet, surfacing as "address already in use" — which otherwise
	// leaves host<->container networking broken until the next restart. Retry a
	// few times so the daemon self-heals once the OS reclaims the stale socket.
	wgConfig := wgtypes.Config{
		ListenPort: &listenPort,
		PrivateKey: hostPrivateKey,
		Peers:      []wgtypes.PeerConfig{peer},
	}
	const maxConfigAttempts = 6
	for attempt := 1; ; attempt++ {
		err = wgClient.ConfigureDevice(interfaceName, wgConfig)
		if err == nil {
			break
		}
		if attempt >= maxConfigAttempts || !strings.Contains(err.Error(), "address already in use") {
			return fmt.Errorf("Failed to configure Wireguard device: %w", err)
		}
		dockerInstrInfof("WireGuard :%d still in use (attempt %d/%d), retrying in 1s: %v",
			listenPort, attempt, maxConfigAttempts, err)
		time.Sleep(time.Second)
	}

	for _, config := range network.IPAM.Config {
		if network.Scope == "local" {
			// Best-effort delete any pre-existing route for this subnet before
			// adding ours. A stale route left behind by a node that didn't exit
			// cleanly (e.g. one still pointing at a now-dead utun) makes the add
			// below fail with "File exists" AND keeps blackholing the container
			// subnet. Deleting first makes the route setup idempotent and lets a
			// fresh node reclaim the subnet. The error is ignored because "no
			// such route" is the normal, expected case.
			_ = exec.Command("route", "-q", "-n", "delete", "-inet", config.Subnet).Run()

			cmd := exec.Command("route", "-q", "-n", "add", "-inet", config.Subnet, "-interface", interfaceName)

			outputBytes, err := cmd.CombinedOutput()

			if err != nil {
				return fmt.Errorf("Failed to add route: %w, %s", err, string(outputBytes))
			}

		}
	}

	// Wait for program to terminate
	<-wgDevice.Wait()

	return nil
}

// createTunIfNotExists returns nil if a tun device already exists; otherwise returns the created device
func createTunIfNotExists(
	ctx context.Context,
) (tun.Device, error) {
	// always attempt to create to avoid races
	tunDevice, err := tun.CreateTUN("utun", device.DefaultMTU)
	if err != nil {
		return nil, fmt.Errorf("Failed to create TUN device: %w", err)
	}

	interfaceName, err := tunDevice.Name()
	if err != nil {
		return nil, fmt.Errorf("Failed to get TUN device name: %w", err)
	}

	cmd := exec.Command("ifconfig", interfaceName, "inet", hostPeerCIDR, vmPeerIp)

	outputBytes, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("Failed to set interface address with ifconfig: %w, %s", err, string(outputBytes))
	}

	tunIndex, err := strconv.Atoi(strings.TrimPrefix(interfaceName, UTUN))
	if err != nil {
		return nil, err
	}

	lowestTunIndex, err := getCurrentTunIndex(
		context.Background(),
	)
	if nil != err {
		return nil, err
	}

	if lowestTunIndex < tunIndex {
		// we got raced and lost..
		tunDevice.Close()
		return nil, nil
	}

	return tunDevice, nil
}

// getCurrentTunIndex retrieves the lowest index of any existing utun interface on the system
func getCurrentTunIndex(
	ctx context.Context,
) (int, error) {

	lowestTunIndex := 1000000

	interfaces, err := net.Interfaces()
	if err != nil {
		return lowestTunIndex, fmt.Errorf("Failed to list interfaces: %w", err)
	}

	for _, i := range interfaces {
		if !strings.HasPrefix(i.Name, UTUN) {
			continue
		}

		iTunIndex, err := strconv.Atoi(strings.TrimPrefix(i.Name, UTUN))
		if err != nil {
			continue
		}

		addrs, err := i.Addrs()
		if err != nil {
			return lowestTunIndex, err
		}

		for _, a := range addrs {
			if a.String() == hostPeerCIDR {
				// we got raced and lost...
				if lowestTunIndex > iTunIndex {
					lowestTunIndex = iTunIndex
				}
			}
		}
	}

	return lowestTunIndex, nil
}

func setupVm(
	ctx context.Context,
	dockerClient dockerClientPkg.CommonAPIClient,
	serverPort int,
	hostPeerIp string,
	vmPeerIp string,
	hostPrivateKey wgtypes.Key,
	vmPrivateKey wgtypes.Key,
) error {
	imageRef := "ghcr.io/chipmk/docker-mac-net-connect/setup:v0.1.3"

	err := pullImage(
		ctx,
		&model.ContainerCall{
			Image: &model.ContainerCallImage{
				Ref: &imageRef,
			},
		},
		dockerClient,
		"",
		noOpEventPublisher{},
	)
	if err != nil {
		return err
	}

	containerName := getContainerName(uuid.NewV4().String(), "wireguard-setup")

	defer dockerClient.ContainerRemove(
		context.Background(),
		containerName,
		container.RemoveOptions{
			RemoveVolumes: true,
			Force:         true,
		},
	)

	resp, err := dockerClient.ContainerCreate(
		ctx,
		&container.Config{
			Image: imageRef,
			Env: []string{
				"SERVER_PORT=" + strconv.Itoa(serverPort),
				"HOST_PEER_IP=" + hostPeerIp,
				"VM_PEER_IP=" + vmPeerIp,
				"HOST_PUBLIC_KEY=" + hostPrivateKey.PublicKey().String(),
				"VM_PRIVATE_KEY=" + vmPrivateKey.String(),
			},
		},
		&container.HostConfig{
			AutoRemove:  true,
			NetworkMode: "host",
			CapAdd: []string{
				"NET_ADMIN",
			},
		},
		nil,
		nil,
		containerName,
	)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	// Run container to completion
	err = dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{})
	if err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	reader, err := dockerClient.ContainerLogs(ctx, resp.ID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		return fmt.Errorf("failed to get logs for container %s: %w", resp.ID, err)
	}

	defer reader.Close()

	_, err = stdcopy.StdCopy(io.Discard, os.Stderr, reader)
	if err != nil {
		return err
	}

	return nil
}

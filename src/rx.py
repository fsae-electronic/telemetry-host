import serial
import msgpack
import argparse
import logging
from cobs import cobs

logger = logging.getLogger(__name__)

def receive_packet(ser):
    """
    Waits for, reads, and decodes a single COBS-framed MessagePack packet.
    Returns the unpacked dictionary, or None if the frame was empty or corrupted.
    """
    raw_frame = ser.read_until(b'\x00')
    
    if not raw_frame or raw_frame == b'\x00':
        return None

    # Only spend time formatting the hex string if debug logging is actually on
    if logger.isEnabledFor(logging.DEBUG):
        hex_dump = ' '.join(f'{b:02X}' for b in raw_frame)
        logger.debug(f"Raw frame: {hex_dump}")

    try:
        # Strip the 0x00 delimiter
        frame_to_decode = raw_frame[:-1]
        
        msgpack_payload = cobs.decode(frame_to_decode)
        unpacked_data = msgpack.unpackb(msgpack_payload)
        
        if isinstance(unpacked_data, dict):
            return unpacked_data
        else:
            logger.debug("Unpacked data is not a dictionary.")
            return None

    except cobs.DecodeError:
        logger.debug("Dropped packet: Corrupted COBS frame.")
        return None
    except msgpack.exceptions.ExtraData:
        logger.debug("Dropped packet: Invalid Msgpack payload.")
        return None
    except Exception as e:
        logger.debug(f"Unexpected error: {e}")
        return None


def main():
    parser = argparse.ArgumentParser(description="Framed Msgpack Telemetry Receiver")
    parser.add_argument('-p', '--port', type=str, default='/dev/ttyUSB0', 
                        help='Serial port to listen on (default: /dev/ttyUSB0)')
    parser.add_argument('-b', '--baud', type=int, default=115200, 
                        help='Baud rate (default: 115200)')
    parser.add_argument('-v', '--verbose', action='store_true', 
                        help='Print formatted sensor data as it arrives')
    parser.add_argument('-d', '--debug', action='store_true', 
                        help='Enable debug logging (prints raw bytes and errors)')
    
    args = parser.parse_args()

    # Set the logging level based on the argparse flag
    log_level = logging.DEBUG if args.debug else logging.INFO
    
    # Configure the format for the console output
    logging.basicConfig(
        level=log_level,
        format='%(asctime)s [%(levelname)s] %(message)s',
        datefmt='%H:%M:%S'
    )

    logger.info(f"Starting Framed Msgpack receiver on {args.port} at {args.baud} baud...")
    
    try:
        with serial.Serial(args.port, args.baud, timeout=1) as ser:
            logger.info("Device successfully opened and ready. Listening for data...")
            
            while True:
                # The function signature is now completely clean
                packet = receive_packet(ser)
                
                # We can still use standard print for the actual data payload if verbose is set,
                # keeping it cleanly separated from the system log messages.
                if packet and args.verbose:
                    print("\n--- Valid Sensor Packet ---")
                    for sensor_name, value in packet.items():
                        key_str = sensor_name.decode('utf-8') if isinstance(sensor_name, bytes) else sensor_name
                        print(f"  {key_str}: {value:.2f}")
                        
    except serial.SerialException as e:
        logger.error(f"Failed to open serial port: {e}")
    except KeyboardInterrupt:
        logger.info("Receiver stopped by user.")

if __name__ == '__main__':
    main()

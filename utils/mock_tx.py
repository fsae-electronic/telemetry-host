import serial
import msgpack
import argparse
import logging
import time
import math
from cobs import cobs

logger = logging.getLogger(__name__)

NUM_SENSORS = 12

def generate_mock_data(counter):
    """
    Generates a dictionary containing mock data for 12 sensors.
    Uses sine waves to simulate realistic, smoothly changing ADC values.
    """
    data = {}
    for i in range(NUM_SENSORS):
        # Create a simulated value that oscillates (e.g., simulating suspension travel or temperatures)
        # We offset the phase based on the sensor index so they don't all look identical
        val = math.sin(counter + (i * 0.5)) * 100.0 + 50.0
        # Format the key to match the C code's style (e.g., 's_00', 's_01')
        data[f's_{i:02d}'] = round(val, 2)
    return data

def send_packet(ser, data_dict):
    """
    Packs a dictionary into MessagePack, COBS-encodes it, frames it, 
    and sends it out over the provided serial port.
    """
    try:
        # Pack the dictionary into binary MessagePack format
        # use_bin_type=True ensures strings are packed efficiently
        msgpack_payload = msgpack.packb(data_dict, use_bin_type=True)
        
        cobs_encoded = cobs.encode(msgpack_payload)
        
        # Append the absolute frame delimiter (0x00)
        frame = cobs_encoded + b'\x00'
        
        # Send it out over the UART
        ser.write(frame)
        
        # Only spend time formatting the hex string if debug logging is actually on
        if logger.isEnabledFor(logging.DEBUG):
            hex_dump = ' '.join(f'{b:02X}' for b in frame)
            logger.debug(f"Transmitted raw frame: {hex_dump}")
            
        return True
        
    except Exception as e:
        logger.error(f"Failed to send packet: {e}")
        return False

def main():
    parser = argparse.ArgumentParser(description="Framed Msgpack Telemetry Transmitter (Mock MCU)")
    parser.add_argument('-p', '--port', type=str, default='/dev/ttyUSB1', 
                        help='Serial port to transmit on (default: /dev/ttyUSB1)')
    parser.add_argument('-b', '--baud', type=int, default=115200, 
                        help='Baud rate (default: 115200)')
    parser.add_argument('-r', '--rate', type=float, default=10.0, 
                        help='Transmission rate in Hz (default: 10.0 Hz)')
    parser.add_argument('-v', '--verbose', action='store_true', 
                        help='Print formatted sensor data as it is sent')
    parser.add_argument('-d', '--debug', action='store_true', 
                        help='Enable debug logging (prints raw bytes and system info)')
    
    args = parser.parse_args()

    
    # Configure the format for the console output
    log_level = logging.DEBUG if args.debug else logging.INFO
    logging.basicConfig(
        level=log_level,
        format='%(asctime)s [%(levelname)s] %(message)s',
        datefmt='%H:%M:%S'
    )

    logger.info(f"Starting Mock MCU Transmitter on {args.port} at {args.baud} baud ({args.rate} Hz)...")
    
    sleep_time = 1.0 / args.rate
    counter = 0.0

    try:
        with serial.Serial(args.port, args.baud, timeout=1) as ser:
            logger.info("Device successfully opened. Starting transmission loop...")
            
            while True:
                # Generate new state
                mock_data = generate_mock_data(counter)
                
                # Transmit state
                success = send_packet(ser, mock_data)
                
                # Handle verbose output independent of system logs
                if success and args.verbose:
                    print("\n--- Transmitted Mock Packet ---")
                    for key, val in mock_data.items():
                        print(f"  {key}: {val:.2f}")
                        
                # Increment the counter to evolve the sine waves and sleep
                counter += 0.1
                time.sleep(sleep_time)

    except serial.SerialException as e:
        logger.error(f"Failed to open serial port: {e}")
    except KeyboardInterrupt:
        logger.info("Transmitter stopped by user.")

if __name__ == '__main__':
    main()

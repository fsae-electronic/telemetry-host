# FSAE ECU Telemetry Host
This repository contains the host-side telemetry code. To handle the unreliability of physical RF serial streams, the protocol relies on **MessagePack** for lightweight binary serialization and **COBS (Consistent Overhead Byte Stuffing)** for robust packet framing and recovery.

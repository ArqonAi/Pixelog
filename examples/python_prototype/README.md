# Python Smart Bootleg Prototype

This directory contains a Proof-of-Concept (PoC) python script demonstrating how to apply Pixelog-style "data-in-video" concepts to audio metadata.

## Concept

Instead of using the full Go-based Pixelog binary for archival, this script demonstrates a lightweight approach for **"Smart Bootlegs"**:
- **Audio:** Remains as the primary audio track (e.g., a concert recording).
- **Video:** The video track is generated to contain synchronized QR codes.
- **Data:** The QR codes carry machine-readable JSON payloads (lyrics, trivia, AI analysis) that change over time.

## Usage

1. Install dependencies:
   ```bash
   pip install opencv-python qrcode[pil] numpy
   ```

2. Run the script:
   ```bash
   python smart_bootleg.py
   ```

3. Output:
   The script generates `output/cornell_77_smart_bootleg.mp4`, a video file with embedded metadata frames.

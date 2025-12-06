import cv2
import qrcode
import numpy as np
import json
from PIL import Image, ImageDraw, ImageFont
import os

def create_smart_bootleg_prototype():
    # 1. The Payload (The "Smart" part)
    # In a real app, this would change every second based on the audio timestamp
    payload = {
        "event": "Grateful Dead - Cornell University",
        "date": "1977-05-08",
        "venue": "Barton Hall",
        "track": "Scarlet Begonias",
        "timestamp": "00:01:23",
        "ai_analysis": {
            "mood": "Euphoric",
            "key": "B Mixolydian",
            "trivia": "This transition to Fire on the Mountain is widely considered the greatest of all time."
        }
    }
    
    payload_str = json.dumps(payload, indent=2)
    print(f"Encoding payload:\n{payload_str}")

    # 2. Setup Video Writer
    output_dir = "output"
    os.makedirs(output_dir, exist_ok=True)
    output_path = os.path.join(output_dir, "cornell_77_smart_bootleg.mp4")
    
    width, height = 1280, 720
    fps = 30
    duration_sec = 5
    
    # MP4V codec is widely supported
    fourcc = cv2.VideoWriter_fourcc(*'mp4v') 
    video = cv2.VideoWriter(output_path, fourcc, fps, (width, height))

    if not video.isOpened():
        print(f"Error: Could not create video writer for {output_path}")
        return

    # 3. Generate QR Code
    qr = qrcode.QRCode(
        version=1,
        error_correction=qrcode.constants.ERROR_CORRECT_H, # High error correction
        box_size=10,
        border=4,
    )
    qr.add_data(payload_str)
    qr.make(fit=True)
    qr_img_pil = qr.make_image(fill_color="black", back_color="white").convert('RGB')

    # 4. Create Frame (Using PIL for easier text drawing, then converting to OpenCV)
    # Create background
    base_frame = Image.new('RGB', (width, height), color=(20, 20, 20)) # Dark gray bg
    draw = ImageDraw.Draw(base_frame)
    
    # Paste QR Code in center
    qr_pos = ((width - qr_img_pil.width) // 2, (height - qr_img_pil.height) // 2)
    base_frame.paste(qr_img_pil, qr_pos)

    # Draw Text
    try:
        # Try to load a font, fallback to default
        font_large = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 40)
        font_small = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 20)
    except IOError:
        font_large = ImageFont.load_default()
        font_small = ImageFont.load_default()

    # Title
    draw.text((50, 50), "Deadhead-LLM: Smart Bootleg Prototype", font=font_large, fill=(255, 255, 255))
    draw.text((50, 100), "Scan Center Code for AI Knowledge Graph Data", font=font_small, fill=(200, 200, 200))

    # Metadata Overlay
    draw.text((50, height - 100), f"Track: {payload['track']}", font=font_small, fill=(0, 255, 0))
    draw.text((50, height - 70), f"Analysis: {payload['ai_analysis']['mood']}", font=font_small, fill=(0, 255, 0))

    # Convert PIL image to OpenCV format (numpy array)
    # PIL is RGB, OpenCV is BGR
    frame_np = np.array(base_frame)
    frame_bgr = cv2.cvtColor(frame_np, cv2.COLOR_RGB2BGR)

    # 5. Write Frames
    print(f"Generating {duration_sec} seconds of video...")
    for _ in range(fps * duration_sec):
        video.write(frame_bgr)

    video.release()
    print(f"Success! Video saved to: {output_path}")
    print("Concept: This video contains machine-readable knowledge embedded in the visual stream.")

if __name__ == "__main__":
    create_smart_bootleg_prototype()

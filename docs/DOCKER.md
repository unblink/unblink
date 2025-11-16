# Self-Hosting with Docker

This guide provides instructions for setting up and running Unblink using Docker. This is the recommended method for self-hosting as it simplifies deployment and dependency management.

## Quick Start (Recommended)

Instead of cloning and rebuilding the image, you can use the latest released Docker image directly:

**Option 1: Simple Docker command**
```bash
docker run -d --name unblink -p 3000:3000 -v unblink-data:/data/unblink -e APPDATA=/data tri2820/unblink:latest
```

**Option 2: Using Docker Compose**
Create a `docker-compose.yml` file with the following content:

```yaml
version: '3.8'
services:
  unblink:
    image: tri2820/unblink:latest
    ports:
      - "3000:3000"
    environment:
      - PORT=3000
      - HOSTNAME=0.0.0.0
      - APPDATA=/data
    volumes:
      - unblink-data:/data/unblink
    restart: unless-stopped
volumes:
  unblink-data:
```

Then run:
```bash
docker-compose up -d
```

Access the application at `http://localhost:3000` in your web browser.

## Building from Source (Advanced)

If you prefer to build the Docker image from source code (for development or customization):

1.  **Prerequisites**

    - [Docker](https://docs.docker.com/get-docker/) installed on your system.
    - [Docker Compose](https://docs.docker.com/compose/install/) installed on your system.
    - [Git](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git) for cloning the repository.

2.  **Clone the Repository**

    Open your terminal and clone the Unblink repository to your local machine:

    ```bash
    git clone https://github.com/tri2820/unblink
    cd unblink
    ```

3.  **Build and Start the Container**

    Use `docker-compose` to build the Docker image and run the container in detached mode (`-d`):

    ```bash
    docker-compose up --build -d
    ```

    -   `--build`: This flag forces the rebuilding of the Docker image, which is useful when you've made changes to the source code or `Dockerfile`.
    -   `-d`: This runs the container in the background.

## Data Persistence

The `docker-compose.yml` file is configured to use a named Docker volume (`unblink-data`) to persist application data. This ensures that your configuration, database, and other data are not lost when the container is stopped, removed, or recreated.

The application data inside the container is stored at `/data/unblink`, which is mapped to the `unblink-data` volume on your host machine.

## Configuration

You can customize the application's configuration by modifying the `environment` section of the `docker-compose.yml` file.

| Environment Variable | Description                                                                                                                                                           | Default Value   |
| -------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------- |
| `PORT`               | The port on which the application will run inside the container.                                                                                                      | `3000`          |
| `HOSTNAME`           | The hostname the application will bind to. `0.0.0.0` makes it accessible from outside the container.                                                                    | `0.0.0.0`       |
| `APPDATA`            | The directory where the application stores its data inside the container.                                                                                             | `/data`         |
| `ENGINE_URL`         | The hostname of the Unblink AI inference engine. By default, it uses the public engine. You can change this to point to your self-hosted instance of `unblink-engine`. | `api.zapdoslabs.com` |

To change a variable, open `docker-compose.yml` and modify the value. For example, to change the `ENGINE_URL`:

```yaml
services:
  unblink:
    # ...
    environment:
      - PORT=3000
      - HOSTNAME=0.0.0.0
      - APPDATA=/data
      - ENGINE_URL=your-engine-hostname.com
    # ...
```

After making changes, you need to restart the container for them to take effect:

```bash
docker-compose down
docker-compose up -d
```

## Updating Your Installation

To update your Unblink installation to the latest version:

1.  **Pull the latest changes** from the Git repository:

    ```bash
    git pull
    ```

2.  **Rebuild and restart** your container with `docker-compose`:

    ```bash
    docker-compose up --build -d
    ```

## Stopping the Application

To stop the Unblink container, run:

```bash
docker-compose down
```

This will stop and remove the container, but the `unblink-data` volume will be preserved.



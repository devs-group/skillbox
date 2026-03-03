"""Python client for the Skillbox API — a secure skill execution runtime for AI agents.

Zero dependencies beyond the Python standard library.

Quick start::

    from skillbox import Client

    client = Client("http://localhost:8080", "sk-your-api-key")
    result = client.run("data-analysis", input={"data": [1, 2, 3]})
    print(result.status, result.output)

Authentication:
    Pass the API key directly, or leave it empty to read from the
    SKILLBOX_API_KEY environment variable.

Multi-tenancy:
    Use the ``tenant_id`` parameter to scope all requests to a tenant::

        client = Client(base_url, api_key, tenant_id="tenant-42")
"""

from __future__ import annotations

import gzip
import io
import json
import mimetypes
import os
import tarfile
import uuid
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, BinaryIO
from urllib.error import HTTPError, URLError
from urllib.parse import urljoin
from urllib.request import Request, urlopen


# ------------------------------------------------------------------
# Types
# ------------------------------------------------------------------


@dataclass
class RunResult:
    """Result returned after a skill execution completes."""

    execution_id: str = ""
    status: str = ""
    output: Any = None
    files_url: str = ""
    files_list: list[str] = field(default_factory=list)
    logs: str = ""
    duration_ms: int = 0
    error: str = ""

    @property
    def has_files(self) -> bool:
        """Whether the execution produced downloadable output files."""
        return bool(self.files_url)


@dataclass
class Skill:
    """A registered skill definition as returned by list endpoints."""

    name: str = ""
    version: str = ""
    description: str = ""
    lang: str = ""


@dataclass
class SkillDetail:
    """Full skill metadata returned by get_skill, including the SKILL.md
    instructions body. This is the key data structure for agents that need
    to understand what a skill does before executing it."""

    name: str = ""
    version: str = ""
    description: str = ""
    lang: str = ""
    image: str = ""
    instructions: str = ""
    timeout: str = ""
    resources: dict[str, str] = field(default_factory=dict)


@dataclass
class FileInfo:
    """A file record from the Skillbox API."""

    id: str = ""
    tenant_id: str = ""
    session_id: str = ""
    execution_id: str = ""
    name: str = ""
    content_type: str = ""
    size_bytes: int = 0
    s3_key: str = ""
    version: int = 0
    parent_id: str | None = None
    created_at: str = ""
    updated_at: str = ""


class APIError(Exception):
    """Raised when the Skillbox API responds with a non-2xx status code."""

    def __init__(
        self,
        status_code: int,
        error_code: str = "",
        message: str = "",
    ) -> None:
        self.status_code = status_code
        self.error_code = error_code
        self.message = message
        super().__init__(str(self))

    def __str__(self) -> str:
        if self.message:
            return f"skillbox: {self.status_code} {self.error_code}: {self.message}"
        if self.error_code:
            return f"skillbox: {self.status_code} {self.error_code}"
        return f"skillbox: {self.status_code}"


# ------------------------------------------------------------------
# Client
# ------------------------------------------------------------------


class Client:
    """Communicates with the Skillbox API.

    Create one with :class:`Client` and reuse it — it is safe for
    concurrent use from multiple threads.

    Args:
        base_url: The Skillbox server URL (e.g. ``http://localhost:8080``).
        api_key: API key for authentication. Falls back to the
            ``SKILLBOX_API_KEY`` environment variable when empty.
        tenant_id: Optional tenant ID sent via ``X-Tenant-ID`` header.
        timeout: HTTP request timeout in seconds. ``None`` means no timeout.
    """

    def __init__(
        self,
        base_url: str,
        api_key: str = "",
        *,
        tenant_id: str = "",
        timeout: float | None = None,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key or os.environ.get("SKILLBOX_API_KEY", "")
        self.tenant_id = tenant_id
        self.timeout = timeout

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def run(
        self,
        skill: str,
        *,
        version: str = "",
        input: Any = None,
        env: dict[str, str] | None = None,
    ) -> RunResult:
        """Execute a skill and block until completion.

        Args:
            skill: Name of the registered skill (e.g. ``"data-analysis"``).
            version: Pin a specific version. Empty uses latest.
            input: JSON-serialisable payload forwarded to the skill.
            env: Extra environment variables injected into the container.

        Returns:
            The full :class:`RunResult` including output, logs, and file
            metadata.
        """
        body: dict[str, Any] = {"skill": skill}
        if version:
            body["version"] = version
        if input is not None:
            body["input"] = input
        if env:
            body["env"] = env

        data = self._request("POST", "/v1/executions", body=body)
        return _parse_run_result(data)

    def get_execution(self, execution_id: str) -> RunResult:
        """Retrieve the current state of a previously started execution."""
        data = self._request("GET", f"/v1/executions/{execution_id}")
        return _parse_run_result(data)

    def get_execution_logs(self, execution_id: str) -> str:
        """Return combined stdout/stderr logs for an execution."""
        raw = self._request_raw("GET", f"/v1/executions/{execution_id}/logs")
        # Try JSON envelope first; fall back to raw text.
        try:
            envelope = json.loads(raw)
            if isinstance(envelope, dict) and envelope.get("logs"):
                return envelope["logs"]
        except (json.JSONDecodeError, TypeError):
            pass
        return raw.decode() if isinstance(raw, bytes) else raw

    def register_skill(self, zip_path: str) -> None:
        """Upload a skill zip archive to the Skillbox server.

        Args:
            zip_path: Path to a readable ``.zip`` file on disk.
        """
        path = Path(zip_path)
        if not path.is_file():
            raise FileNotFoundError(f"skillbox: skill archive not found: {zip_path}")

        boundary = uuid.uuid4().hex
        content_type = f"multipart/form-data; boundary={boundary}"

        with open(path, "rb") as f:
            file_data = f.read()

        body = _build_multipart(boundary, path.name, file_data)

        req = self._build_request("POST", "/v1/skills")
        req.add_header("Content-Type", content_type)
        req.data = body

        resp = self._do(req)
        if resp.status < 200 or resp.status >= 300:
            raise _parse_api_error(resp)
        resp.read()  # drain

    def list_skills(self) -> list[Skill]:
        """Return all skills registered on the server.

        The response includes descriptions so callers can decide which
        skill to use.
        """
        data = self._request("GET", "/v1/skills")
        if not isinstance(data, list):
            return []
        return [
            Skill(
                name=s.get("name", ""),
                version=s.get("version", ""),
                description=s.get("description", ""),
                lang=s.get("lang", ""),
            )
            for s in data
        ]

    def get_skill(self, name: str, version: str = "latest") -> SkillDetail:
        """Retrieve the full metadata for a specific skill version.

        This includes the SKILL.md instructions body — use it to understand
        what a skill does, what input it expects, and how it behaves before
        calling :meth:`run`.

        Args:
            name: The skill name (e.g. ``"data-analysis"``).
            version: The skill version (e.g. ``"1.0.0"``). Defaults to
                ``"latest"``.

        Returns:
            A :class:`SkillDetail` with the full metadata and instructions.
        """
        data = self._request("GET", f"/v1/skills/{name}/{version}")
        if not isinstance(data, dict):
            return SkillDetail()
        return SkillDetail(
            name=data.get("name", ""),
            version=data.get("version", ""),
            description=data.get("description", ""),
            lang=data.get("lang", ""),
            image=data.get("image", ""),
            instructions=data.get("instructions", ""),
            timeout=data.get("timeout", ""),
            resources=data.get("resources") or {},
        )

    # ------------------------------------------------------------------
    # File management
    # ------------------------------------------------------------------

    def list_files(
        self,
        *,
        session_id: str = "",
        execution_id: str = "",
        limit: int = 50,
        offset: int = 0,
    ) -> list[FileInfo]:
        """List files, optionally filtered by session or execution.

        Args:
            session_id: Filter to files belonging to this session.
            execution_id: Filter to files belonging to this execution.
            limit: Maximum number of results to return.
            offset: Number of results to skip for pagination.

        Returns:
            A list of :class:`FileInfo` objects.
        """
        params: list[str] = []
        if session_id:
            params.append(f"session_id={session_id}")
        if execution_id:
            params.append(f"execution_id={execution_id}")
        params.append(f"limit={limit}")
        params.append(f"offset={offset}")

        qs = "&".join(params)
        data = self._request("GET", f"/v1/files?{qs}")
        if not isinstance(data, list):
            return []
        return [_parse_file_info(f) for f in data]

    def get_file(self, file_id: str) -> FileInfo:
        """Retrieve metadata for a single file.

        Args:
            file_id: The file UUID.

        Returns:
            A :class:`FileInfo` with the file metadata.
        """
        data = self._request("GET", f"/v1/files/{file_id}")
        return _parse_file_info(data)

    def download_file(self, file_id: str, dest_path: str) -> None:
        """Download a file's content and write it to disk.

        The destination path is validated to prevent directory traversal.

        Args:
            file_id: The file UUID.
            dest_path: Local filesystem path to write the file to.

        Raises:
            ValueError: If dest_path attempts directory traversal.
        """
        abs_dest = os.path.abspath(dest_path)
        cwd = os.path.abspath(".")
        # Ensure dest_path does not escape via traversal.  We allow
        # any absolute path that is not formed by ".." components.
        if ".." in os.path.normpath(dest_path).split(os.sep):
            raise ValueError(
                f"skillbox: path traversal detected in dest_path: {dest_path}"
            )

        raw = self._request_raw("GET", f"/v1/files/{file_id}/download")
        parent = os.path.dirname(abs_dest)
        if parent:
            os.makedirs(parent, exist_ok=True)
        with open(abs_dest, "wb") as f:
            f.write(raw)

    def update_file(self, file_id: str, file_path: str) -> FileInfo:
        """Upload a new version of an existing file.

        Args:
            file_id: The file UUID to update.
            file_path: Path to the replacement file on disk.

        Returns:
            The updated :class:`FileInfo`.
        """
        path = Path(file_path)
        if not path.is_file():
            raise FileNotFoundError(
                f"skillbox: file not found: {file_path}"
            )

        boundary = uuid.uuid4().hex
        content_type = f"multipart/form-data; boundary={boundary}"

        with open(path, "rb") as f:
            file_data = f.read()

        body = _build_multipart(boundary, path.name, file_data)

        req = self._build_request("PUT", f"/v1/files/{file_id}")
        req.add_header("Content-Type", content_type)
        req.data = body

        try:
            resp = self._do(req)
        except HTTPError as e:
            raise _parse_api_error(e) from e

        resp_body = resp.read()
        if not resp_body:
            return FileInfo()

        try:
            data = json.loads(resp_body)
        except json.JSONDecodeError:
            return FileInfo()

        return _parse_file_info(data)

    def delete_file(self, file_id: str) -> None:
        """Delete a file.

        The server returns 204 No Content on success.

        Args:
            file_id: The file UUID to delete.
        """
        req = self._build_request("DELETE", f"/v1/files/{file_id}")

        try:
            resp = self._do(req)
        except HTTPError as e:
            raise _parse_api_error(e) from e

        resp.read()  # drain

    def list_file_versions(self, file_id: str) -> list[FileInfo]:
        """List all versions of a file.

        Args:
            file_id: The file UUID.

        Returns:
            A list of :class:`FileInfo` representing each version.
        """
        data = self._request("GET", f"/v1/files/{file_id}/versions")
        if not isinstance(data, list):
            return []
        return [_parse_file_info(f) for f in data]

    # ------------------------------------------------------------------
    # Health
    # ------------------------------------------------------------------

    def health(self) -> None:
        """Check whether the Skillbox server is reachable.

        Raises:
            APIError: If the server returns a non-2xx response.
        """
        self._request("GET", "/health")

    def download_files(self, result: RunResult, dest_dir: str) -> None:
        """Fetch and extract the output file archive from an execution.

        If the execution produced no files, this is a no-op.

        All tar entry paths are validated to prevent path-traversal attacks.

        Args:
            result: The :class:`RunResult` whose files to download.
            dest_dir: Local directory to extract files into.
        """
        if not result.has_files:
            return

        req = Request(result.files_url, method="GET")
        try:
            resp = urlopen(req, timeout=self.timeout)
        except HTTPError as e:
            raise APIError(e.code, message=f"download files: HTTP {e.code}") from e
        except URLError as e:
            raise APIError(0, message=f"download files: {e.reason}") from e

        if resp.status < 200 or resp.status >= 300:
            raise APIError(resp.status, message=f"download files: HTTP {resp.status}")

        _extract_tar_gz(resp, dest_dir)

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    def _build_request(self, method: str, path: str) -> Request:
        url = f"{self.base_url}{path}"
        req = Request(url, method=method)
        if self.api_key:
            req.add_header("Authorization", f"Bearer {self.api_key}")
        if self.tenant_id:
            req.add_header("X-Tenant-ID", self.tenant_id)
        return req

    def _do(self, req: Request):
        """Execute a request, returning the raw response."""
        try:
            return urlopen(req, timeout=self.timeout)
        except HTTPError:
            raise
        except URLError as e:
            raise APIError(0, message=f"{req.get_method()} {req.full_url}: {e.reason}") from e

    def _request(self, method: str, path: str, *, body: Any = None) -> Any:
        """Make an API request and return decoded JSON."""
        req = self._build_request(method, path)

        if body is not None:
            req.add_header("Content-Type", "application/json")
            req.data = json.dumps(body).encode()

        try:
            resp = self._do(req)
        except HTTPError as e:
            raise _parse_api_error(e) from e

        resp_body = resp.read()
        if not resp_body:
            return None

        try:
            return json.loads(resp_body)
        except json.JSONDecodeError:
            return resp_body.decode()

    def _request_raw(self, method: str, path: str) -> bytes:
        """Make an API request and return raw bytes."""
        req = self._build_request(method, path)

        try:
            resp = self._do(req)
        except HTTPError as e:
            raise _parse_api_error(e) from e

        return resp.read()


# ------------------------------------------------------------------
# Module-level helpers
# ------------------------------------------------------------------


def _parse_run_result(data: Any) -> RunResult:
    """Convert a JSON dict to a RunResult."""
    if not isinstance(data, dict):
        return RunResult()
    return RunResult(
        execution_id=data.get("execution_id", ""),
        status=data.get("status", ""),
        output=data.get("output"),
        files_url=data.get("files_url", "") or "",
        files_list=data.get("files_list") or [],
        logs=data.get("logs", "") or "",
        duration_ms=data.get("duration_ms", 0) or 0,
        error=data.get("error", "") or "",
    )


def _parse_file_info(data: Any) -> FileInfo:
    """Convert a JSON dict to a FileInfo."""
    if not isinstance(data, dict):
        return FileInfo()
    return FileInfo(
        id=data.get("id", ""),
        tenant_id=data.get("tenant_id", ""),
        session_id=data.get("session_id", ""),
        execution_id=data.get("execution_id", ""),
        name=data.get("name", ""),
        content_type=data.get("content_type", ""),
        size_bytes=data.get("size_bytes", 0) or 0,
        s3_key=data.get("s3_key", ""),
        version=data.get("version", 0) or 0,
        parent_id=data.get("parent_id"),
        created_at=data.get("created_at", ""),
        updated_at=data.get("updated_at", ""),
    )


def _parse_api_error(resp) -> APIError:
    """Parse an HTTP error response into an APIError."""
    if isinstance(resp, HTTPError):
        status_code = resp.code
        body = resp.read()
    else:
        status_code = resp.status
        body = resp.read()

    error_code = ""
    message = ""

    if body:
        try:
            data = json.loads(body)
            if isinstance(data, dict):
                error_code = data.get("error", "")
                message = data.get("message", "")
        except (json.JSONDecodeError, TypeError):
            message = body.decode(errors="replace").strip()

    return APIError(status_code, error_code, message)


def _build_multipart(boundary: str, filename: str, data: bytes) -> bytes:
    """Build a minimal multipart/form-data body."""
    lines = [
        f"--{boundary}".encode(),
        f'Content-Disposition: form-data; name="file"; filename="{filename}"'.encode(),
        b"Content-Type: application/zip",
        b"",
        data,
        f"--{boundary}--".encode(),
        b"",
    ]
    return b"\r\n".join(lines)


def _extract_tar_gz(fileobj: BinaryIO, dest_dir: str) -> None:
    """Decompress a gzip stream and extract the tar archive into dest_dir.

    Validates every entry path to prevent directory traversal.
    """
    abs_dest = os.path.abspath(dest_dir)

    raw = fileobj.read()
    gz = gzip.GzipFile(fileobj=io.BytesIO(raw))

    with tarfile.open(fileobj=gz, mode="r|") as tf:
        for member in tf:
            target = os.path.normpath(os.path.join(abs_dest, member.name))
            # Prevent path traversal.
            if not (
                target == abs_dest
                or target.startswith(abs_dest + os.sep)
            ):
                raise ValueError(
                    f"skillbox: path traversal detected in tar entry: {member.name}"
                )

            if member.isdir():
                os.makedirs(target, exist_ok=True)
            elif member.isfile():
                os.makedirs(os.path.dirname(target), exist_ok=True)
                extracted = tf.extractfile(member)
                if extracted:
                    with open(target, "wb") as out:
                        out.write(extracted.read())
